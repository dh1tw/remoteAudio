package cmd

import "strings"

// nats does not allow certain characters to be used as a subject (topic) name
// validateNatsSubject will return a validated & sanitized subject string
func validateSubject(topic string) string {
	return strings.Replace(topic, " ", "_", -1)
}
