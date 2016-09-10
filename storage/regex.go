package storage

import "regexp"

type remap struct {
	Regex       *regexp.Regexp
	Replacement string
}

type Regex struct {
	remaps []remap
}

func NewRegexFromList(redirects map[string]string) (*Regex, error) {
	remaps := make([]remap, 0, len(redirects))

	for regexString, redirect := range redirects {
		r, err := regexp.Compile(regexString)
		if err != nil {
			return nil, err
		}
		remaps = append(remaps, remap{
			Regex:       r,
			Replacement: redirect,
		})
	}

	return &Regex{
		remaps: remaps,
	}, nil
}

func (r Regex) Load(rawShort string) (string, error) {
	short, err := sanitizeShort(rawShort)
	if err != nil {
		return "", err
	}

	for _, remap := range r.remaps {
		if remap.Regex.MatchString(short) {
			return remap.Regex.ReplaceAllString(short, remap.Replacement), nil
		}
	}

	return "", ErrShortNotSet
}
