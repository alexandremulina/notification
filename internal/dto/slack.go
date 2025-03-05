package dto

import (
	"fmt"
	"strings"
	"time"
)

// ConsentDocument represents a consent document
type ConsentDocument struct {
	ConsentID        string    `json:"consentId"`
	Status           string    `json:"status"`
	CreationDateTime time.Time `json:"creationDateTime"`
	CreatedAt        time.Time `json:"createdAt"`
	Permissions      []string  `json:"permissions"`
}

// ContactInfo represents contact information
type ContactInfo struct {
	Email string `json:"email"`
	Phone string `json:"phone"`
}

// SlackBlock represents a Slack block
type SlackBlock struct {
	Type     string        `json:"type"`
	Text     *SlackText    `json:"text,omitempty"`
	Fields   []SlackText   `json:"fields,omitempty"`
	Elements []interface{} `json:"elements,omitempty"`
}

// SlackText represents text in a Slack block
type SlackText struct {
	Type  string `json:"type"`
	Text  string `json:"text"`
	Emoji bool   `json:"emoji,omitempty"`
}

// SlackButton represents a button in a Slack block
type SlackButton struct {
	Type  string    `json:"type"`
	Text  SlackText `json:"text"`
	Value string    `json:"value"`
	URL   string    `json:"url,omitempty"`
}

// SlackNotification represents a Slack notification
type SlackNotification struct {
	Text   string       `json:"text"`
	Blocks []SlackBlock `json:"blocks"`
}

// FormatPhoneNumber formats a phone number for display
func FormatPhoneNumber(phone string) string {
	// Simple formatting - remove any non-digit characters and format as needed
	phone = strings.TrimSpace(phone)
	if strings.HasPrefix(phone, "+") {
		return phone // Already formatted with country code
	}
	return phone
}

// CreateSlackConsentNotification creates a Slack notification for a consent document
func CreateSlackConsentNotification(consent ConsentDocument, userID, tenantID string, contactInfo *ContactInfo) SlackNotification {
	// Extract short IDs for display
	consentShortID := consent.ConsentID
	if parts := strings.Split(consent.ConsentID, ":"); len(parts) > 0 {
		consentShortID = parts[len(parts)-1]
		if len(consentShortID) > 8 {
			consentShortID = consentShortID[:8]
		}
	} else if len(consentShortID) > 8 {
		consentShortID = consentShortID[:8]
	}

	userShortID := userID
	if len(userShortID) > 8 {
		userShortID = userShortID[:8]
	}

	// Format created date
	var createdDate string
	if !consent.CreationDateTime.IsZero() {
		createdDate = consent.CreationDateTime.Format("Jan 2, 3:04 PM")
	} else if !consent.CreatedAt.IsZero() {
		createdDate = consent.CreatedAt.Format("Jan 2, 3:04 PM")
	} else {
		createdDate = time.Now().Format("Jan 2, 3:04 PM")
	}

	// Format permissions for display
	keyPermissions := "No permissions requested"
	if len(consent.Permissions) > 0 {
		permList := make([]string, 0, len(consent.Permissions))
		for i, perm := range consent.Permissions {
			if i < 3 {
				permList = append(permList, "• "+perm)
			}
		}

		if len(consent.Permissions) > 3 {
			permList = append(permList, fmt.Sprintf("• +%d more", len(consent.Permissions)-3))
		}

		keyPermissions = strings.Join(permList, "\n")
	}

	// Build action elements array
	actionElements := make([]interface{}, 0)

	// Add email button if email is provided
	if contactInfo != nil && contactInfo.Email != "" {
		actionElements = append(actionElements, SlackButton{
			Type: "button",
			Text: SlackText{
				Type:  "plain_text",
				Text:  "📧 Email User",
				Emoji: true,
			},
			Value: "email_user",
			URL:   fmt.Sprintf("mailto:%s", contactInfo.Email),
		})
	}

	// Always add View Details button
	actionElements = append(actionElements, SlackButton{
		Type: "button",
		Text: SlackText{
			Type:  "plain_text",
			Text:  "View Details",
			Emoji: true,
		},
		Value: "view_details",
		URL:   fmt.Sprintf("https://example.com/consents/%s", consentShortID),
	})

	// Create blocks array
	blocks := []SlackBlock{
		{
			Type: "header",
			Text: &SlackText{
				Type:  "plain_text",
				Text:  fmt.Sprintf("🔔 New Consent Created: %s", consentShortID),
				Emoji: true,
			},
		},
		{
			Type: "divider",
		},
		{
			Type: "section",
			Fields: []SlackText{
				{
					Type: "mrkdwn",
					Text: fmt.Sprintf("*👤 User ID:*\n%s", userShortID),
				},
				{
					Type: "mrkdwn",
					Text: fmt.Sprintf("*🔹 Status:*\n%s", consent.Status),
				},
			},
		},
	}

	// Add contact information section if available
	if contactInfo != nil {
		var emailText, phoneText string
		if contactInfo.Email != "" {
			emailText = fmt.Sprintf("Email: %s\n", contactInfo.Email)
		}
		if contactInfo.Phone != "" {
			phoneText = fmt.Sprintf("Phone: %s", FormatPhoneNumber(contactInfo.Phone))
		}

		blocks = append(blocks, SlackBlock{
			Type: "section",
			Fields: []SlackText{
				{
					Type: "mrkdwn",
					Text: fmt.Sprintf("*✉️ Contact:*\n%s%s", emailText, phoneText),
				},
				{
					Type: "mrkdwn",
					Text: fmt.Sprintf("*🕒 Created:*\n%s\n*🌐 Environment:* %s", createdDate, tenantID),
				},
			},
		})
	} else {
		blocks = append(blocks, SlackBlock{
			Type: "section",
			Fields: []SlackText{
				{
					Type: "mrkdwn",
					Text: fmt.Sprintf("*🕒 Created:*\n%s", createdDate),
				},
				{
					Type: "mrkdwn",
					Text: fmt.Sprintf("*🌐 Environment:*\n%s", tenantID),
				},
			},
		})
	}

	// Add permissions section
	blocks = append(blocks, SlackBlock{
		Type: "section",
		Text: &SlackText{
			Type: "mrkdwn",
			Text: fmt.Sprintf("*🔑 Permissions:*\n%s", keyPermissions),
		},
	})

	// Add context section
	blocks = append(blocks, SlackBlock{
		Type: "context",
		Elements: []interface{}{
			SlackText{
				Type: "mrkdwn",
				Text: fmt.Sprintf("Consent ID: `%s`", consent.ConsentID),
			},
		},
	})

	// Add actions section
	blocks = append(blocks, SlackBlock{
		Type:     "actions",
		Elements: actionElements,
	})

	return SlackNotification{
		Text:   fmt.Sprintf("New Consent Created: %s", consentShortID),
		Blocks: blocks,
	}
}
