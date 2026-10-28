package config

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLoadCORSOrigins(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://one.example,https://two.example")
	assert.Equal(t, []string{"https://one.example", "https://two.example"}, Load().CORSAllowedOrigins)
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
	assert.Equal(t, []string{"*"}, Load().CORSAllowedOrigins)
}
