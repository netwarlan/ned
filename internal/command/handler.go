package command

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
)

const maxMessageLen = 1500

// respondNow sends an immediate text response (no deferred "thinking..." state).
func respondNow(s *discordgo.Session, i *discordgo.InteractionCreate, content string, ephemeral bool) {
	flags := discordgo.MessageFlags(0)
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: content, Flags: flags},
	}); err != nil {
		log.Printf("Error sending response: %v", err)
	}
}

// respondDeferred sends an ephemeral "thinking..." response that gives us
// up to 15 minutes to reply.
func respondDeferred(s *discordgo.Session, i *discordgo.InteractionCreate, ephemeral bool) {
	flags := discordgo.MessageFlags(0)
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Flags: flags},
	}); err != nil {
		log.Printf("Error deferring response: %v", err)
	}
}

// followUp edits the deferred response with a text message.
func followUp(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
	}); err != nil {
		log.Printf("Error editing response: %v", err)
	}
}

// followUpEmbed edits the deferred response with a rich embed.
func followUpEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, embeds []*discordgo.MessageEmbed) {
	if _, err := s.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Embeds: &embeds,
	}); err != nil {
		log.Printf("Error editing response with embed: %v", err)
	}
}

// followUpError edits the deferred response with an error message.
func followUpError(s *discordgo.Session, i *discordgo.InteractionCreate, msg string, err error) {
	content := fmt.Sprintf("**Error:** %s", msg)
	if err != nil {
		content += fmt.Sprintf("\n```\n%s\n```", truncate(err.Error(), 500))
	}
	followUp(s, i, content)
}

// truncate shortens a string to maxLen, appending "... (truncated)" if needed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... (truncated)"
}
