package bot

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"unicode"

	"github.com/DzyubSpirit/reTGanslatorBot/config"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type BotAPI interface {
	AnswerInlineQuery(config tgbotapi.InlineConfig) (tgbotapi.APIResponse, error)
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
}

type Handler struct {
	bot    BotAPI
	config config.Config
}

func NewHandler(config config.Config, bot BotAPI) *Handler {
	return &Handler{
		bot:    bot,
		config: config,
	}
}

func hasChatTag(chatName, text string) bool {
	return strings.Contains(strings.ToLower(text), "*"+strings.ToLower(chatName))
}

func (bh Handler) inlineQuery(update tgbotapi.Update) {
	query := *update.InlineQuery
	aliases := bh.config.AllAliases()
	for i, alias := range aliases {
		aliases[i] = "*" + alias
	}
	matched := aliases
	words := strings.Fields(query.Query)
	withoutLastWord := query.Query
	if len(words) > 0 {
		matched = nil
		lastWord := words[len(words)-1]
		withoutLastWord = strings.TrimRightFunc(query.Query, unicode.IsSpace)[0 : len(query.Query)-len(lastWord)]
		for _, alias := range aliases {
			if strings.Contains(strings.ToLower(alias), strings.ToLower(lastWord)) {
				matched = append(matched, alias)
			}
		}
	}
	var results []interface{}
	for _, alias := range matched {
		fullSuggestion := withoutLastWord + alias
		results = append(results, tgbotapi.NewInlineQueryResultArticle(fullSuggestion, alias, fullSuggestion))
	}
	inlineConfig := tgbotapi.InlineConfig{
		InlineQueryID: query.ID,
		Results:       results,
	}
	bh.bot.AnswerInlineQuery(inlineConfig)
}

func (bh Handler) message(update tgbotapi.Update) {
	log.Printf("[%s] text: %s, caption: %s", update.Message.From.UserName, update.Message.Text, update.Message.Caption)
	if update.Message.Entities != nil {
		for _, entity := range *update.Message.Entities {
			username := update.Message.Text[entity.Offset : entity.Offset+entity.Length]
			if entity.Type == "mention" && username == "@reTGanslatorBot" {
				aliases := bh.config.AllAliases()
				for i, alias := range aliases {
					aliases[i] = "*" + strings.ToLower(alias)
				}
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Tags: "+strings.Join(aliases, " "))
				msg.BaseChat.ReplyToMessageID = update.Message.MessageID
				bh.bot.Send(msg)
			}
		}
	}

	found := false
	for _, chat := range bh.config.AllChats() {
		if update.Message.Chat.ID == chat.ID {
			found = true
			break
		}
	}
	if !found {
		return
	}

	for _, chat := range bh.config.AllChats() {
		hasTags := false
		for _, alias := range chat.Aliases {
			if hasChatTag(alias, update.Message.Text) || hasChatTag(alias, update.Message.Caption) {
				hasTags = true
				break
			}
		}
		if !hasTags {
			continue
		}

		{
			msg := tgbotapi.NewMessage(chat.ID, "Пересилаю повідомлення з чату "+update.Message.Chat.Title)
			bh.bot.Send(msg)
		}
		if update.Message.ReplyToMessage != nil {
			msg := tgbotapi.NewForward(chat.ID, update.Message.Chat.ID, update.Message.ReplyToMessage.MessageID)
			bh.bot.Send(msg)
		}
		{
			msg := tgbotapi.NewForward(chat.ID, update.Message.Chat.ID, update.Message.MessageID)
			bh.bot.Send(msg)
		}
	}
}

func (bh Handler) command(update tgbotapi.Update) {
	msg := update.Message
	if msg.CommandWithAt() != msg.Command() {
		cmd := msg.CommandWithAt()
		i := strings.Index(cmd, "@")
		at := cmd[i+1:]
		if at != "reTGanslatorBot" {
			return
		}
	}
	if msg.Command() != "help" {
		return
	}

	aliases := bh.config.AllAliases()
	for i := range aliases {
		aliases[i] = "*" + strings.ToLower(aliases[i])
	}
	aliasesStr := strings.Join(aliases, " ")
	contactsStr := strings.Join(bh.config.HelpContacts, " ")
	newMsg := tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf(`
Щоб переслати повідомлення в інший UACT чат:

(а) Ще не відправлене повідомлення
Додайте до тексту повідомлення тег під час його написання.

(б) Вже відправлене повідомлення
Відповідайте на потрібне повідомлення і додавайте тег до тексту відповіді.
Бот надішле обидва повідомлення в потрібний чат(и): (1) те, на яке ви відповідаєте і (2) безпосередньо вашу відповідь з тегом.

Щоб побачити доступні теги, почніть писати повідомлення в будь-якому чаті UACT з @reTGanslator, і бот запропонує вам список тегів. Також можна тегнути бота у будь-якому повідомленні, і бот надішле список усіх тегів.
Доступні такі теги:
%s

За поясненням до тегів і як працює пересилка, звертайтеся до %s
	`, aliasesStr, contactsStr))
	bh.bot.Send(newMsg)
}

func (bh Handler) HandleUpdate(update tgbotapi.Update) error {
	var err error
	switch {
	case update.InlineQuery != nil:
		bh.inlineQuery(update)
	case update.Message != nil && update.Message.IsCommand():
		bh.command(update)
	case update.Message != nil:
		bh.message(update)
	default:
		err = errors.New("unknown type of message")
	}
	return err
}
