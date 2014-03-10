package storytime

import (
	"fmt"

	"appengine"
	"appengine/mail"
)

const (
	serverUrl string = "http://storytime.brieandsteve.com/story/%s/%s"
	sender           = "Storytime <storytime@storytime-brieandsteve.appspot.com>"
)

// Sends an email to the author of part with a link to continue.
func sendMail(c appengine.Context, story Story) {
	if story.Complete {
		return
	}
	var subject, text string
	part := story.LastPart()
	url := fmt.Sprintf(serverUrl, story.Id, story.NextId)
	if part != nil {
		subject = "Write the next part of this story."
		text = fmt.Sprintf("%s, %s wrote:\n> %s\n\nPlease visit %s to write the next part.",
			fuzzyTime(part.Written), getFullEmail(c, part.Author), part.Visible, url)
	} else {
		subject = "Write the first part of this story."
		text = fmt.Sprintf("%s, %s initiated a new story.\n\nPlease visit %s to write the beginning.",
			fuzzyTime(part.Written), getFullEmail(c, story.Creator), url)
	}

	msg := &mail.Message{
		Sender:  sender,
		To:      []string{story.NextAuthor},
		Subject: subject,
		Body:    text,
	}
	if err := mail.Send(c, msg); err != nil {
		c.Errorf("Couldn't send email: %v", err)
		panic(err)
	}
}

// Sends an email only if this story is the author's current story.
func maybeSendMail(c appengine.Context, story Story) {
	if story.Complete {
		return
	}
	author := story.NextAuthor
	current := currentStory(c, author)
	if current.Id == story.Id {
		sendMail(c, story)
	}
}
