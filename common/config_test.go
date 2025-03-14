package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigDefaultsForEnv(t *testing.T) {
	for _, env := range []Environment{Development, Stagenet, Mainnet} {
		conf := ConfigDefaultsForEnv(env)
		require.Equal(t, env, conf.Env)
		// testing for pointer inequality, each call returns a new instance
		require.True(t, conf != ConfigDefaultsForEnv(env))
	}
}
