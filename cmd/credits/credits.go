package credits

import (
	_ "embed"
	"fmt"
)

//go:embed CREDITS
var credits []byte

func Do() error {
	fmt.Println(string(credits))
	return nil
}
