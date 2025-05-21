// Copyright 2020 Snowfork
// SPDX-License-Identifier: LGPL-3.0-only

package solochain_test

import (
	"testing"

	"github.com/snowfork/snowbridge/relayer/chain/solochain"
	"github.com/stretchr/testify/assert"
)

func TestMortalEra(t *testing.T) {
	era := solochain.NewMortalEra(1)
	assert.Equal(t, era.AsMortalEra.First, byte(21))
	assert.Equal(t, era.AsMortalEra.Second, byte(0))

	era = solochain.NewMortalEra(63)
	assert.Equal(t, era.AsMortalEra.First, byte(245))
	assert.Equal(t, era.AsMortalEra.Second, byte(3))

	era = solochain.NewMortalEra(64)
	assert.Equal(t, era.AsMortalEra.First, byte(5))
	assert.Equal(t, era.AsMortalEra.Second, byte(0))
}
