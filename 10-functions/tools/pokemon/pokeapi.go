package pokemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// pokemonResponse is the struct that represents the response from the PokeAPI.
// We are only interested in the id, name, moves and types.
type pokemonResponse struct {
	Id    int    `json:"id"`
	Name  string `json:"name"`
	Moves []struct {
		Move struct {
			Name string `json:"name"`
		} `json:"move"`
	} `json:"moves"`
	Types []struct {
		Type struct {
			Name string `json:"name"`
		} `json:"type"`
	} `json:"types"`
}

// FetchAPI fetches the pokemon information from PokeAPI. It returns a string with the pokemon information,
// including the ID, the number of moves, the moves and the types.
func FetchAPI(ctx context.Context, pokemon string) (s string, err error) {
	baseApiUrl := fmt.Sprintf("https://pokeapi.co/api/v2/pokemon/%s", strings.ToLower(pokemon))

	req, err := http.NewRequestWithContext(ctx, "GET", baseApiUrl, nil)
	if err != nil {
		return "", err
	}

	req.Header.Add("User-Agent", "pokemon-tool")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return "", fmt.Errorf("copying data in pokeapi: %w", err)
	}

	var p pokemonResponse
	err = json.Unmarshal(buf.Bytes(), &p)
	if err != nil {
		return "", fmt.Errorf("unmarshalling data in pokeapi: %w", err)
	}
	defer resp.Body.Close()

	var moveNames []string
	for _, m := range p.Moves {
		moveNames = append(moveNames, m.Move.Name)
	}

	var typeNames []string
	for _, t := range p.Types {
		typeNames = append(typeNames, t.Type.Name)
	}

	return fmt.Sprintf("ID: %d, Name: %s, MovesCount: %d, Moves: [%s], Types: [%s]", p.Id, p.Name, len(moveNames), strings.Join(moveNames, ", "), strings.Join(typeNames, ", ")), nil
}
