package config

import (
	"testing"
)

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"empty string", "", 100, ""},
		{"whitespace trimmed", "  hello  ", 100, "hello"},
		{"control chars stripped", "hel\x00lo\x01\x02", 100, "hello"},
		{"max length enforced", "hello world", 5, "hello"},
		{"DEL stripped", "hello\x7Fworld", 100, "helloworld"},
		{"tab control char", "hello\tworld", 100, "helloworld"},
		{"unicode preserved", "héllo wörld", 100, "héllo wörld"},
		{"no change needed", "clean-string", 100, "clean-string"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeString(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("SanitizeString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizeComment(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"empty", "", 100, ""},
		{"newline preserved", "line1\nline2", 100, "line1\nline2"},
		{"tab preserved", "col1\tcol2", 100, "col1\tcol2"},
		{"control char stripped", "hello\x00world", 100, "helloworld"},
		{"non-printable stripped", "hello\x01\x02world", 100, "helloworld"},
		{"max length", "hello world", 5, "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeComment(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("SanitizeComment() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSanitizeCLIInput(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"normal string unchanged", "hello", "hello"},
		{"backticks stripped", "`rm -rf /`", "rm -rf /"},
		{"dollar stripped", "$VARIABLE", "VARIABLE"},
		{"braces stripped", "{cmd}", "cmd"},
		{"parens stripped", "(subshell)", "subshell"},
		{"semicolon stripped", "cmd;evil", "cmdevil"},
		{"pipe stripped", "cmd|evil", "cmdevil"},
		{"ampersand stripped", "cmd&", "cmd"},
		{"angle brackets stripped", "<tag>", "tag"},
		{"backslash stripped", "C:\\path", "C:path"},
		{"quotes stripped", "'string' \"string\"", "string string"},
		{"mixed injection", "hello; $(rm -rf /)", "hello rm -rf /"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeCLIInput(tt.input, 100)
			if got != tt.want {
				t.Errorf("SanitizeCLIInput() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCoercePropertyValue(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		v, err := CoercePropertyValue("  hello  ", SchemaString)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != "hello" {
			t.Errorf("got %v, want 'hello'", v)
		}
		_, err = CoercePropertyValue(42, SchemaString)
		if err == nil {
			t.Error("expected error for non-string to string")
		}
	})

	t.Run("integer", func(t *testing.T) {
		v, err := CoercePropertyValue("42", SchemaInteger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != 42 {
			t.Errorf("got %v, want 42", v)
		}
		v, err = CoercePropertyValue(42, SchemaInteger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != 42 {
			t.Errorf("got %v, want 42", v)
		}
		v, err = CoercePropertyValue(42.5, SchemaInteger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != 42 {
			t.Errorf("got %v, want 42", v)
		}
		_, err = CoercePropertyValue("not-a-number", SchemaInteger)
		if err == nil {
			t.Error("expected error for invalid integer string")
		}
	})

	t.Run("boolean", func(t *testing.T) {
		for _, s := range []string{"true", "TRUE", "yes", "1"} {
			v, err := CoercePropertyValue(s, SchemaBoolean)
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", s, err)
			}
			if v != true {
				t.Errorf("got %v, want true for %q", v, s)
			}
		}
		for _, s := range []string{"false", "FALSE", "no", "0"} {
			v, err := CoercePropertyValue(s, SchemaBoolean)
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", s, err)
			}
			if v != false {
				t.Errorf("got %v, want false for %q", v, s)
			}
		}
		v, err := CoercePropertyValue(true, SchemaBoolean)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != true {
			t.Errorf("got %v, want true", v)
		}
		_, err = CoercePropertyValue("maybe", SchemaBoolean)
		if err == nil {
			t.Error("expected error for invalid boolean string")
		}
	})

	t.Run("ip address", func(t *testing.T) {
		v, err := CoercePropertyValue("192.168.1.1", SchemaIPAddr)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != "192.168.1.1" {
			t.Errorf("got %v, want '192.168.1.1'", v)
		}
		_, err = CoercePropertyValue("not-an-ip", SchemaIPAddr)
		if err == nil {
			t.Error("expected error for invalid IP")
		}
		_, err = CoercePropertyValue(42, SchemaIPAddr)
		if err == nil {
			t.Error("expected error for non-string IP")
		}
	})

	t.Run("mac address", func(t *testing.T) {
		validMACs := []string{
			"00:11:22:33:44:55",
			"00-11-22-33-44-55",
			"0011.2233.4455",
			"aa:bb:cc:dd:ee:ff",
		}
		for _, mac := range validMACs {
			v, err := CoercePropertyValue(mac, SchemaMACAddr)
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", mac, err)
			}
			if v != mac {
				t.Errorf("got %v, want %q", v, mac)
			}
		}
		_, err := CoercePropertyValue("invalid-mac", SchemaMACAddr)
		if err == nil {
			t.Error("expected error for invalid MAC")
		}
	})

	t.Run("ip prefix", func(t *testing.T) {
		v, err := CoercePropertyValue("192.168.1.0/24", SchemaIPPrefix)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != "192.168.1.0/24" {
			t.Errorf("got %v, want '192.168.1.0/24'", v)
		}
		_, err = CoercePropertyValue("invalid/prefix", SchemaIPPrefix)
		if err == nil {
			t.Error("expected error for invalid CIDR")
		}
	})

	t.Run("nil value", func(t *testing.T) {
		v, err := CoercePropertyValue(nil, SchemaString)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if v != nil {
			t.Errorf("got %v, want nil", v)
		}
	})
}

func TestValidateEntryID(t *testing.T) {
	t.Run("empty is valid", func(t *testing.T) {
		err := ValidateEntryID("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("valid UUID", func(t *testing.T) {
		err := ValidateEntryID("550e8400-e29b-41d4-a716-446655440000")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invalid UUID", func(t *testing.T) {
		err := ValidateEntryID("not-a-uuid")
		if err == nil {
			t.Fatal("expected error for invalid UUID")
		}
	})
}

func TestValidateNumbers(t *testing.T) {
	t.Run("empty is valid", func(t *testing.T) {
		err := ValidateNumbers(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("valid numbers", func(t *testing.T) {
		err := ValidateNumbers([]string{"0", "1", "2", "5"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("valid range", func(t *testing.T) {
		err := ValidateNumbers([]string{"0-5", "10-20"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("invalid number", func(t *testing.T) {
		err := ValidateNumbers([]string{"abc"})
		if err == nil {
			t.Fatal("expected error for invalid number")
		}
	})

	t.Run("negative number", func(t *testing.T) {
		err := ValidateNumbers([]string{"-1"})
		if err == nil {
			t.Fatal("expected error for negative number")
		}
	})

	t.Run("invalid range", func(t *testing.T) {
		err := ValidateNumbers([]string{"5-0"})
		if err == nil {
			t.Fatal("expected error for invalid range (start > end)")
		}
	})

	t.Run("empty element", func(t *testing.T) {
		err := ValidateNumbers([]string{""})
		if err == nil {
			t.Fatal("expected error for empty element")
		}
	})
}

func TestValidateWhere(t *testing.T) {
	schema := &ModuleSchema{
		Path: "/test",
		Schema: map[string]*SchemaProperty{
			"name":    {Type: SchemaString},
			"enabled": {Type: SchemaBoolean},
		},
	}

	t.Run("valid where", func(t *testing.T) {
		err := ValidateWhere(map[string]interface{}{
			"name":    "test",
			"enabled": true,
		}, schema)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("non-existent property", func(t *testing.T) {
		err := ValidateWhere(map[string]interface{}{
			"nonexistent": "value",
		}, schema)
		if err == nil {
			t.Fatal("expected error for non-existent property")
		}
	})

	t.Run("empty key", func(t *testing.T) {
		err := ValidateWhere(map[string]interface{}{
			"": "value",
		}, schema)
		if err == nil {
			t.Fatal("expected error for empty key")
		}
	})
}

func TestValidateFlags(t *testing.T) {
	schema := &ModuleSchema{
		Path: "/test",
		Flags: []SchemaFlag{
			{Letter: "X", Name: "disabled"},
			{Letter: "D", Name: "dynamic"},
		},
	}

	t.Run("valid flags", func(t *testing.T) {
		err := ValidateFlags(map[string]bool{"disabled": true}, schema)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("unknown flag", func(t *testing.T) {
		err := ValidateFlags(map[string]bool{"nonexistent": true}, schema)
		if err == nil {
			t.Fatal("expected error for unknown flag")
		}
	})
}

func TestValidMACFormat(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"colon format", "00:11:22:33:44:55", true},
		{"dash format", "00-11-22-33-44-55", true},
		{"dot format", "0011.2233.4455", true},
		{"uppercase", "AA:BB:CC:DD:EE:FF", true},
		{"mixed case", "aA:bB:cC:dD:eE:fF", true},
		{"too short", "00:11:22:33:44", false},
		{"invalid chars", "00:11:22:33:44:GG", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validMACFormat(tt.input)
			if got != tt.valid {
				t.Errorf("validMACFormat(%q) = %v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}
