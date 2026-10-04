package memstore

import (
	"testing"

	"golearn/internal/link"
	"golearn/internal/storetest"
)

func TestStore(t *testing.T) {
	storetest.Run(t, func(t *testing.T) link.Store {
		return New()
	})
}
