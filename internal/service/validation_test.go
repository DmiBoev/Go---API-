package service

import (
	"dept-api/internal/domain"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateString(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
		invalid           bool
	}{
		{name: "trim", input: "  Sales  ", want: "Sales"},
		{name: "empty", input: "  ", invalid: true},
		{name: "too long", input: strings.Repeat("a", 201), invalid: true},
		{name: "unicode characters", input: strings.Repeat("Я", 200), want: strings.Repeat("Я", 200)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateString(tc.input, 200)
			if tc.invalid {
				assert.ErrorIs(t, err, domain.ErrInvalid)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}
