package main

import "testing"

func TestContactWithoutTelegram(t *testing.T) {
	for _, value := range []string{"@valid_name", "Нет Telegram", "Нет Telegram; Связь: WhatsApp +82 10-1111-2222"} {
		if !validTelegramChoice(value) {
			t.Fatalf("valid contact rejected: %q", value)
		}
	}
	for _, value := range []string{"Нет Telegram; Связь: ", "Нет Telegram; Связь: \nsecret", "WhatsApp only"} {
		if validTelegramChoice(value) {
			t.Fatalf("invalid contact accepted: %q", value)
		}
	}
}
