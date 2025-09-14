package main

import (
	"fmt"
	"regexp"
)

func main() {
	pattern := `^[0-9]+[kMGT]?B?$`
	values := []string{"4GB", "16MB", "128MB", "64MB", "4MB"}
	
	re := regexp.MustCompile(pattern)
	for _, v := range values {
		fmt.Printf("%s matches? %v\n", v, re.MatchString(v))
	}
}
