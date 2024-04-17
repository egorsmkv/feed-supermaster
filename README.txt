Changes:

- Remove unnecessary stuff (see the mother repo - https://github.com/umputun/feed-master - for that stuff)
- Use a telegram bot to publish news to a group

Get the chat ID value using the command /chat_id after adding the bot to a group.

Build:

    go build -ldflags="-s -w" -o feed-master app/main.go
