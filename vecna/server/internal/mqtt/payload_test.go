package mqtt

import "testing"

func TestDecodeDiscovery(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr bool
		want    DiscoveryPayload
	}{
		{
			name: "dimmable device",
			raw:  `{"unique_id":"plug-1","name":"Office Plug","state_topic":"cync_lan/plug-1/state","command_topic":"cync_lan/plug-1/set","brightness":true}`,
			want: DiscoveryPayload{UniqueID: "plug-1", Name: "Office Plug", StateTopic: "cync_lan/plug-1/state", CommandTopic: "cync_lan/plug-1/set", Brightness: true},
		},
		{
			name: "on/off only device",
			raw:  `{"unique_id":"plug-2","name":"Lamp Plug","state_topic":"cync_lan/plug-2/state","command_topic":"cync_lan/plug-2/set"}`,
			want: DiscoveryPayload{UniqueID: "plug-2", Name: "Lamp Plug", StateTopic: "cync_lan/plug-2/state", CommandTopic: "cync_lan/plug-2/set", Brightness: false},
		},
		{
			name:    "missing unique_id",
			raw:     `{"name":"No ID","state_topic":"x","command_topic":"y"}`,
			wantErr: true,
		},
		{
			name:    "missing topics",
			raw:     `{"unique_id":"plug-3","name":"No Topics"}`,
			wantErr: true,
		},
		{
			name:    "malformed json",
			raw:     `{not json`,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodeDiscovery([]byte(tc.raw))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestDecodeState(t *testing.T) {
	cases := []struct {
		name       string
		raw        string
		wantErr    bool
		wantState  string
		wantBright *int
	}{
		{
			name:       "on with brightness",
			raw:        `{"state":"ON","brightness":180}`,
			wantState:  "ON",
			wantBright: func() *int { v := 180; return &v }(),
		},
		{
			name:      "off without brightness field",
			raw:       `{"state":"OFF"}`,
			wantState: "OFF",
		},
		{
			name:    "missing state",
			raw:     `{"brightness":100}`,
			wantErr: true,
		},
		{
			name:    "malformed json",
			raw:     `not json at all`,
			wantErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodeState([]byte(tc.raw))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.State != tc.wantState {
				t.Fatalf("got state %q, want %q", got.State, tc.wantState)
			}
			if (got.Brightness == nil) != (tc.wantBright == nil) {
				t.Fatalf("brightness presence mismatch: got %v, want %v", got.Brightness, tc.wantBright)
			}
			if got.Brightness != nil && tc.wantBright != nil && *got.Brightness != *tc.wantBright {
				t.Fatalf("got brightness %d, want %d", *got.Brightness, *tc.wantBright)
			}
		})
	}
}

func TestEncodePowerCommand(t *testing.T) {
	cases := []struct {
		on   bool
		want string
	}{
		{on: true, want: `{"state":"ON"}`},
		{on: false, want: `{"state":"OFF"}`},
	}
	for _, tc := range cases {
		got := string(EncodePowerCommand(tc.on))
		if got != tc.want {
			t.Fatalf("EncodePowerCommand(%v) = %s, want %s", tc.on, got, tc.want)
		}
	}
}

func TestEncodeBrightnessCommand(t *testing.T) {
	got := string(EncodeBrightnessCommand(180))
	want := `{"brightness":180,"state":"ON"}`
	if got != want {
		t.Fatalf("EncodeBrightnessCommand(180) = %s, want %s", got, want)
	}
}
