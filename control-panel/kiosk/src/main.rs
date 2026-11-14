// Native kiosk display for control-panel. A real native GUI (Slint), not a
// browser/webview: polls vecna's device API directly
// (https://vecna.charlesragone.com/api/devices, BasicAuth-protected) for the
// list of devices (any deviceType -- light/sound_machine/garage/temp) and
// their current state, and POSTs power toggles back to it -- vecna is the
// single source of truth for both naming and actual control, matching the
// same ~2s poll pattern vecna's own web frontend uses (no WebSocket/SSE on
// that API). There's no local MQTT broker/hardware bridge involved on this
// host at all.

use std::io::Write;
use std::rc::Rc;

use serde::{Deserialize, Serialize};
use slint::{Model, ModelRc, SharedString, VecModel, Weak};

slint::include_modules!();

const POLL_INTERVAL: std::time::Duration = std::time::Duration::from_secs(2);

/// cage doesn't forward the child process's stdout/stderr into systemd's
/// journal for this unit in any observable way, so diagnostics go straight
/// to a file instead.
fn log(msg: &str) {
    if let Ok(mut f) = std::fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open("/tmp/kiosk.log")
    {
        let _ = writeln!(f, "{msg}");
        let _ = f.flush();
    }
}

#[derive(Debug, Clone, Deserialize)]
struct VecnaDevice {
    id: u64,
    name: String,
    #[serde(rename = "deviceType")]
    device_type: String,
    #[serde(rename = "lastPowerState")]
    last_power_state: String,
}

/// Mirrors vecna's own frontend (vecna/src/deviceTypes.ts) exactly -- the
/// emoji is the only thing that varies by type, on/off control is identical.
fn emoji_for(device_type: &str) -> &'static str {
    match device_type {
        "light" => "💡",
        "sound_machine" => "🔊",
        "garage" => "🚪",
        "temp" => "🌡️",
        _ => "❓",
    }
}

#[derive(Debug, Serialize)]
struct PowerRequest {
    on: bool,
}

fn vecna_base_url() -> String {
    std::env::var("VECNA_API_URL")
        .unwrap_or_else(|_| "https://vecna.charlesragone.com".to_string())
}

fn vecna_credentials() -> Option<(String, String)> {
    let user = std::env::var("VECNA_API_USER").ok()?;
    let pass = std::env::var("VECNA_API_PASSWORD").ok()?;
    Some((user, pass))
}

fn apply_auth(builder: reqwest::RequestBuilder) -> reqwest::RequestBuilder {
    match vecna_credentials() {
        Some((user, pass)) => builder.basic_auth(user, Some(pass)),
        None => builder,
    }
}

async fn fetch_devices(client: &reqwest::Client) -> anyhow::Result<Vec<VecnaDevice>> {
    let url = format!("{}/api/devices", vecna_base_url());
    let resp = apply_auth(client.get(&url)).send().await?;
    let devices = resp.error_for_status()?.json::<Vec<VecnaDevice>>().await?;
    Ok(devices)
}

async fn set_power(client: &reqwest::Client, id: u64, on: bool) -> anyhow::Result<()> {
    let url = format!("{}/api/devices/{id}/power", vecna_base_url());
    apply_auth(client.post(&url))
        .json(&PowerRequest { on })
        .send()
        .await?
        .error_for_status()?;
    Ok(())
}

fn sync_zones(window: &MainWindow, devices: &[VecnaDevice]) {
    let model = window.get_zones();
    // Rebuilding wholesale each poll is simpler than diffing and cheap at
    // this scale (a handful of devices, polled every couple seconds).
    let tiles: Vec<ZoneTile> = devices
        .iter()
        .map(|d| ZoneTile {
            zone_id: SharedString::from(d.id.to_string()),
            name: SharedString::from(d.name.as_str()),
            emoji: SharedString::from(emoji_for(&d.device_type)),
            on: d.last_power_state.eq_ignore_ascii_case("on"),
        })
        .collect();
    if model.row_count() == tiles.len() {
        for (i, tile) in tiles.into_iter().enumerate() {
            model.set_row_data(i, tile);
        }
    } else {
        window.set_zones(ModelRc::new(Rc::new(VecModel::from(tiles))));
    }
}

fn main() {
    std::panic::set_hook(Box::new(|info| log(&format!("[kiosk] PANIC: {info}"))));
    log("[kiosk] starting, creating window");

    let window = match MainWindow::new() {
        Ok(w) => w,
        Err(e) => {
            log(&format!("[kiosk] FATAL: MainWindow::new() failed: {e}"));
            std::process::exit(1);
        }
    };
    window.set_zones(ModelRc::new(VecModel::from(Vec::<ZoneTile>::new())));

    if vecna_credentials().is_none() {
        log("[kiosk] WARNING: VECNA_API_USER/VECNA_API_PASSWORD not set, requests will be unauthenticated");
    }

    let client = reqwest::Client::builder()
        .timeout(std::time::Duration::from_secs(5))
        .build()
        .expect("reqwest client");

    {
        let client = client.clone();
        let window_weak = window.as_weak();
        window.on_toggle(move |zone_id| {
            let Some(id) = zone_id.parse::<u64>().ok() else {
                return;
            };
            let Some(window) = window_weak.upgrade() else {
                return;
            };
            // Optimistic update so the tap feels instant; the next poll
            // reconciles with whatever vecna actually reports.
            let model = window.get_zones();
            let mut new_state = false;
            for i in 0..model.row_count() {
                if let Some(mut tile) = model.row_data(i) {
                    if tile.zone_id == zone_id {
                        tile.on = !tile.on;
                        new_state = tile.on;
                        model.set_row_data(i, tile);
                        break;
                    }
                }
            }
            // A fresh thread + one-shot runtime per toggle: simplest way to
            // fire an async request from a plain sync callback (this runs on
            // the Slint event loop thread, which has no tokio runtime of its
            // own) without needing a cross-thread channel for something this
            // infrequent.
            let client = client.clone();
            std::thread::spawn(move || {
                let rt = tokio::runtime::Builder::new_current_thread()
                    .enable_all()
                    .build()
                    .unwrap();
                if let Err(e) = rt.block_on(set_power(&client, id, new_state)) {
                    log(&format!("[kiosk] error: set_power({id}, {new_state}) failed: {e}"));
                }
            });
        });
    }

    // Background tokio runtime owns the polling loop; only the
    // Weak<MainWindow> handle crosses the thread boundary (Slint models
    // aren't Send) -- UI mutation happens inside invoke_from_event_loop.
    let window_weak = window.as_weak();
    std::thread::spawn(move || {
        let rt = tokio::runtime::Builder::new_current_thread()
            .enable_all()
            .build()
            .unwrap();
        rt.block_on(async move {
            let client = reqwest::Client::builder()
                .timeout(std::time::Duration::from_secs(5))
                .build()
                .expect("reqwest client");
            loop {
                match fetch_devices(&client).await {
                    Ok(devices) => {
                        let window_weak = window_weak.clone();
                        let _ = slint::invoke_from_event_loop(move || {
                            if let Some(w) = window_weak.upgrade() {
                                w.set_connected(true);
                                sync_zones(&w, &devices);
                            }
                        });
                    }
                    Err(e) => {
                        log(&format!("[kiosk] error: fetch_devices failed: {e}"));
                        let window_weak = window_weak.clone();
                        let _ = slint::invoke_from_event_loop(move || {
                            if let Some(w) = window_weak.upgrade() {
                                w.set_connected(false);
                            }
                        });
                    }
                }
                tokio::time::sleep(POLL_INTERVAL).await;
            }
        });
    });

    log("[kiosk] entering window.run()");
    match window.run() {
        Ok(()) => log("[kiosk] window.run() returned Ok"),
        Err(e) => log(&format!("[kiosk] error: window.run() returned Err: {e}")),
    }
}
