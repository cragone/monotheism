//! NPC tribes: the tribal world is never empty. A handful of AI-controlled tribes live alongside
//! real players, scaling with how many are connected. An NPC tribe is a normal `Tribe` (just with
//! `npc: true`) driven by calling the same `Tribes::command()` a human client would send, so it
//! can never do anything a player couldn't also do, and `nobody_commands_other_tribes` holds for
//! NPCs too: each only ever issues commands for its own tribe id.
use crate::tribe::{BK, BUILD_RANGE, MAX_QUEUE, RK, Task, Tech, Tribes, UK, dist, wrap};
use crate::util::*;
use serde_json::json;

/// NPC ids live well above anything the server's own connection counter will ever reach
pub(crate) const NPC_ID_BASE: u32 = 1_000_000;
const DECISION_INTERVAL: f64 = 1.5;
const POP_CHECK_INTERVAL: f64 = 30.0;
const MIN_NPCS: usize = 3;
const MAX_NPCS: usize = 10;
const THREAT_RADIUS: f64 = 500.0;
const RAID_MIN_ARMY: usize = 5;
const RAID_RADIUS: f64 = 3000.0;

#[derive(Default)]
pub struct NpcBrain {
    next_decision: f64,
    raid_ready_at: f64,
    was_threatened: bool,
}

fn build_key(kind: BK) -> &'static str {
    match kind {
        BK::Hut => "hut",
        BK::Farm => "farm",
        BK::Barracks => "barracks",
        BK::Tower => "tower",
        BK::Center => "center",
    }
}
fn unit_key(kind: UK) -> &'static str {
    match kind {
        UK::Warrior => "warrior",
        UK::Archer => "archer",
        _ => "villager",
    }
}

/// how many NPC tribes the world should have right now, given how many real tribes are playing
fn npc_target(real_tribes: usize) -> usize {
    (3 + real_tribes / 2).clamp(MIN_NPCS, MAX_NPCS)
}

impl Tribes {
    /// grows the NPC population toward its target, one tribe at a time, throttled by the sim clock
    pub fn manage_npc_population(&mut self) {
        if self.t < self.npc_next_pop_check {
            return;
        }
        self.npc_next_pop_check = self.t + POP_CHECK_INTERVAL;
        let real = self.tribes.values().filter(|t| !t.npc).count();
        let current = self.tribes.values().filter(|t| t.npc).count();
        if current < npc_target(real) {
            let id = self.next_npc_id;
            self.next_npc_id += 1;
            self.add_tribe_internal(id, crate::tribe::TribeCarry::random(id), true);
            self.npc_brains.insert(id, NpcBrain { next_decision: self.t, raid_ready_at: 0.0, was_threatened: false });
        }
    }

    /// runs each NPC tribe's throttled decision pass, and respawns or retires fallen ones
    pub fn step_npcs(&mut self, _dt: f64) {
        let target = npc_target(self.tribes.values().filter(|t| !t.npc).count());
        let due: Vec<u32> = self.npc_brains.iter().filter(|(_, b)| b.next_decision <= self.t).map(|(&id, _)| id).collect();
        for id in due {
            let alive = self.tribes.get(&id).is_some_and(|t| t.alive);
            if !alive {
                let npcs_now = self.tribes.values().filter(|t| t.npc).count();
                if npcs_now <= target {
                    self.respawn(id);
                } else {
                    self.remove_tribe(id);
                    continue; // the brain is already gone with the tribe
                }
            } else {
                self.npc_decide(id);
            }
            if let Some(b) = self.npc_brains.get_mut(&id) {
                b.next_decision = self.t + DECISION_INTERVAL + rand(0.0, 1.0);
            }
        }
    }

    fn npc_decide(&mut self, id: u32) {
        self.npc_recall_if_threatened(id);
        self.npc_economy(id);
        self.npc_build(id);
        self.npc_train(id);
        self.npc_research(id);
        self.npc_raid(id);
    }

    /// pulls idle/gathering villagers back to the Center the first tick an enemy gets close
    fn npc_recall_if_threatened(&mut self, id: u32) {
        let Some((cx, cy)) = self.buildings.iter().find(|b| b.owner == id && b.kind == BK::Center).map(|b| (b.x, b.y)) else { return };
        let threatened = self.units.iter().any(|u| u.owner != id && u.owner != 0 && !u.dead && dist(u.x, u.y, cx, cy) < THREAT_RADIUS);
        let was = self.npc_brains.get(&id).is_some_and(|b| b.was_threatened);
        if threatened && !was {
            let ids: Vec<u32> = self.units.iter().filter(|u| u.owner == id && u.kind == UK::Villager && matches!(u.task, Task::Idle | Task::Gather(_))).map(|u| u.id).collect();
            if !ids.is_empty() {
                self.command(id, &json!({ "t": "move", "ids": ids, "x": cx, "y": cy }));
            }
        }
        if let Some(b) = self.npc_brains.get_mut(&id) {
            b.was_threatened = threatened;
        }
    }

    /// sends idle villagers after whichever resource is furthest below its target stockpile
    fn npc_economy(&mut self, id: u32) {
        let Some(res) = self.tribes.get(&id).map(|t| t.res) else { return };
        let mut wants: Vec<(RK, f64)> = vec![(RK::Bush, 150.0 - res[0]), (RK::Tree, 150.0 - res[1]), (RK::Stone, 100.0 - res[2])];
        wants.sort_by(|a, b| b.1.partial_cmp(&a.1).unwrap());
        let idle: Vec<(u32, f64, f64)> = self.units.iter().filter(|u| u.owner == id && u.kind == UK::Villager && u.task == Task::Idle).map(|u| (u.id, u.x, u.y)).collect();
        for (vid, ux, uy) in idle {
            for &(rk, _) in &wants {
                let nearest = self.nodes.iter().filter(|n| n.kind == rk && n.amount > 0.0).min_by(|a, b| dist(ux, uy, a.x, a.y).partial_cmp(&dist(ux, uy, b.x, b.y)).unwrap()).map(|n| n.id);
                if let Some(nid) = nearest {
                    self.command(id, &json!({ "t": "gather", "ids": [vid], "target": nid }));
                    break;
                }
            }
        }
    }

    /// an ordered checklist: huts for room, then a farm, a barracks, and a tower, cost allowing
    fn npc_build(&mut self, id: u32) {
        let Some(center) = self.buildings.iter().find(|b| b.owner == id && b.kind == BK::Center).map(|b| (b.x, b.y)) else { return };
        let (pop, cap) = (self.pop(id), self.cap(id).max(1));
        let farms = self.buildings.iter().filter(|b| b.owner == id && b.kind == BK::Farm).count();
        let barracks = self.buildings.iter().filter(|b| b.owner == id && b.kind == BK::Barracks).count();
        let towers = self.buildings.iter().filter(|b| b.owner == id && b.kind == BK::Tower).count();
        let villagers = self.units.iter().filter(|u| u.owner == id && u.kind == UK::Villager && !u.dead).count();
        let want = if pop as f64 / cap as f64 > 0.8 {
            Some(BK::Hut)
        } else if farms == 0 && villagers >= 4 {
            Some(BK::Farm)
        } else if barracks == 0 && pop >= 8 {
            Some(BK::Barracks)
        } else if towers == 0 && barracks > 0 {
            Some(BK::Tower)
        } else {
            None
        };
        let Some(kind) = want else { return };
        let Some(t) = self.tribes.get(&id) else { return };
        let cost = kind.stats().cost;
        if (0..3).any(|k| t.res[k] < cost[k]) {
            return;
        }
        let builders: Vec<u32> = self.units.iter().filter(|u| u.owner == id && u.kind == UK::Villager && !u.dead).map(|u| u.id).take(3).collect();
        if builders.is_empty() {
            return;
        }
        for k in 0..24 {
            let a = k as f64 * TAU / 24.0 + rand(0.0, 0.3);
            let (x, y) = (wrap(center.0 + a.cos() * BUILD_RANGE * 0.5), wrap(center.1 + a.sin() * BUILD_RANGE * 0.5));
            self.command(id, &json!({ "t": "build", "ids": builders, "b": build_key(kind), "x": x, "y": y }));
            if self.buildings.iter().any(|b| b.owner == id && b.kind == kind && dist(b.x, b.y, x, y) < 5.0) {
                break;
            }
        }
    }

    /// keeps the army at roughly a third of the population, equipping the best researched weapon
    fn npc_train(&mut self, id: u32) {
        if self.pop(id) >= self.cap(id) {
            return;
        }
        let Some(t) = self.tribes.get(&id) else { return };
        let researched = t.researched.clone();
        let army = self.units.iter().filter(|u| u.owner == id && matches!(u.kind, UK::Warrior | UK::Archer) && !u.dead).count();
        let villagers = self.units.iter().filter(|u| u.owner == id && u.kind == UK::Villager && !u.dead).count();
        if (army as f64) < 0.35 * (army + villagers).max(1) as f64 {
            if let Some(bi) = self.buildings.iter().find(|b| b.owner == id && b.kind == BK::Barracks && b.built >= 1.0 && b.queue.len() < MAX_QUEUE) {
                let bid = bi.id;
                let kind = if rand01() < 0.6 { UK::Warrior } else { UK::Archer };
                let weapon = researched.iter().filter(|w| w.def().unit == kind).max_by(|a, b| a.def().dmg_mult.partial_cmp(&b.def().dmg_mult).unwrap());
                let mut cmd = json!({ "t": "train", "b": bid, "u": unit_key(kind) });
                if let Some(w) = weapon {
                    cmd["weapon"] = json!(w.def().key);
                }
                self.command(id, &cmd);
                return;
            }
        }
        if let Some(bi) = self.buildings.iter().find(|b| b.owner == id && b.kind == BK::Center && b.queue.len() < MAX_QUEUE) {
            let bid = bi.id;
            self.command(id, &json!({ "t": "train", "b": bid, "u": "villager" }));
        }
    }

    /// a fixed priority order through the small weapon tree: cheap melee, then cheap ranged, then upgrades
    fn npc_research(&mut self, id: u32) {
        let Some(t) = self.tribes.get(&id) else { return };
        if t.research.is_some() {
            return;
        }
        for tech in [Tech::Spear, Tech::Bow, Tech::Sword, Tech::Longbow] {
            if t.researched.contains(&tech) {
                continue;
            }
            let def = tech.def();
            if def.requires.is_some_and(|r| !t.researched.contains(&r)) {
                continue;
            }
            if (0..3).any(|k| t.res[k] < def.cost[k]) {
                continue;
            }
            self.command(id, &json!({ "t": "research", "tech": def.key }));
            break;
        }
    }

    /// once the army and larder are big enough, send part of the army at the weakest nearby tribe
    fn npc_raid(&mut self, id: u32) {
        if self.npc_brains.get(&id).is_some_and(|b| self.t < b.raid_ready_at) {
            return;
        }
        let Some(t) = self.tribes.get(&id) else { return };
        if t.res[0] < 100.0 {
            return;
        }
        let Some(center) = self.buildings.iter().find(|b| b.owner == id && b.kind == BK::Center) else { return };
        let (cx, cy) = (center.x, center.y);
        let army: Vec<u32> = self.units.iter().filter(|u| u.owner == id && matches!(u.kind, UK::Warrior | UK::Archer) && !u.dead && u.task == Task::Idle).map(|u| u.id).collect();
        if army.len() < RAID_MIN_ARMY {
            return;
        }
        let mut candidates: Vec<(f64, u32)> = Vec::new(); // (score, enemy Center building id)
        for (&oid, ot) in self.tribes.iter() {
            if oid == id || !ot.alive {
                continue;
            }
            let Some(ocenter) = self.buildings.iter().find(|b| b.owner == oid && b.kind == BK::Center) else { continue };
            if dist(cx, cy, ocenter.x, ocenter.y) > RAID_RADIUS {
                continue;
            }
            let other_army = self.units.iter().filter(|u| u.owner == oid && matches!(u.kind, UK::Warrior | UK::Archer) && !u.dead).count();
            if other_army as f64 >= 0.8 * army.len() as f64 {
                continue;
            }
            candidates.push((ot.score, ocenter.id));
        }
        let Some(&(_, target_bid)) = candidates.iter().min_by(|a, b| a.0.partial_cmp(&b.0).unwrap()) else { return };
        let send_n = ((army.len() as f64) * 0.6).ceil() as usize;
        let send: Vec<u32> = army.into_iter().take(send_n).collect();
        self.command(id, &json!({ "t": "attack", "ids": send, "target": target_bid }));
        if let Some(b) = self.npc_brains.get_mut(&id) {
            b.raid_ready_at = self.t + rand(90.0, 180.0);
        }
    }
}

