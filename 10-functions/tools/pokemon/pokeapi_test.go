package pokemon

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPokeAPI(t *testing.T) {
	output, err := FetchAPI(context.Background(), "pikachu")
	if err != nil {
		t.Fatalf("error calling tool: %v", err)
	}

	require.Contains(t, output, "ID: 25")
	require.Contains(t, output, "MovesCount: 105")
	require.Contains(t, output, "Moves: [")
	require.Contains(t, output, "Types: [")
}
