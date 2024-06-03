package activity

import "regexp"

type Regex *regexp.Regexp

var (
	GenericPercentRegex          Regex = regexp.MustCompile(`(\d+)%`)
	GenericDescriptionColonRegex Regex = regexp.MustCompile(`(.*):[^:]*$`)
)
