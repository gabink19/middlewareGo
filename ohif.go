package main

import (
	"fmt"
)

func GenerateOHIFLink(cfg Config, studyUID string) string {
	return fmt.Sprintf("%s/viewer?studyUID=%s", cfg.OHIFURL, studyUID)
}
