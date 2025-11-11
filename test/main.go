package main

import (
	"fmt"
	"strings"
)


func Key(parts ...string) string {
	prefix := "ayok"
	if prefix == "" {
		prefix = "ayok"
	}

	var sb strings.Builder
	sb.WriteString(prefix)
	for _, part := range parts {
		if part != "" {

			sb.WriteString(":")
			sb.WriteString(part)
		}
	}

	return sb.String()
}

func main(){


	key := Key("123", "yes", "no")
	fmt.Println(key)
	

}