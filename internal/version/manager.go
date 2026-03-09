package version

import (
	"os"
	"strconv"
	"strings"
)

const versionFile = "data/version.txt"

func Get() int {

	data, err := os.ReadFile(versionFile)
	if err != nil {
		return 1
	}

	v, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 1
	}

	return v
}

func Update() int {

	v := Get()
	v++

	os.WriteFile(versionFile, []byte(strconv.Itoa(v)), 0644)

	return v
}