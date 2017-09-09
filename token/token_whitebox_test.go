package token

import (
	"testing"

	"fmt"
	"github.com/fabric8-services/fabric8-auth/resource"
	config "github.com/fabric8-services/fabric8-wit/configuration"
	"github.com/stretchr/testify/require"
)

func TestRemoteTokensLoaded(t *testing.T) {
	c, err := config.GetConfigurationData()
	if err != nil {
		panic(fmt.Errorf("failed to setup the configuration: %s", err.Error()))
	}

	resource.Require(t, resource.Remote)
	m, err := NewManager(c)
	require.Nil(t, err)
	require.NotNil(t, m)
	tm, ok := m.(*tokenManager)
	require.True(t, ok)

	require.NotEmpty(t, tm.PublicKeys())
	require.Equal(t, len(tm.publicKeys), len(m.PublicKeys()))
	require.Equal(t, len(tm.publicKeys), len(tm.publicKeysMap))
	for i, k := range tm.publicKeys {
		require.NotEqual(t, "", k.KID)
		require.NotNil(t, m.PublicKey(k.KID))
		require.Equal(t, m.PublicKeys()[i], k.Key)
	}
}
