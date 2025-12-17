package validation

import "testing"

func TestValidateAppSubtype(t *testing.T) {
	tests := []struct {
		name      string
		subtype   string
		wantError bool
	}{
		{"empty is allowed", "", false},
		{"valid investigations", "investigations", false},
		{"valid opportunities", "opportunities", false},
		{"valid projections", "projections", false},
		{"valid inventory", "inventory", false},
		{"valid events", "events", false},
		{"invalid subtype", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAppSubtype(tt.subtype)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateAppSubtype(%q) error = %v, wantError %v", tt.subtype, err, tt.wantError)
			}
		})
	}
}

func TestValidateAppAndSubtype(t *testing.T) {
	tests := []struct {
		name      string
		app       string
		subtype   string
		wantError bool
	}{
		{"both empty", "", "", false},
		{"app only", "finops", "", false},
		{"subtype without app", "", "investigations", true},
		{"both valid", "finops", "investigations", false},
		{"invalid subtype with app", "finops", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAppAndSubtype(tt.app, tt.subtype)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateAppAndSubtype(%q, %q) error = %v, wantError %v", tt.app, tt.subtype, err, tt.wantError)
			}
		})
	}
}
