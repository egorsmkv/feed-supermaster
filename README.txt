Changes:

- Remove unnecessary stuff (see the mother repo - https://github.com/umputun/feed-master - for that stuff)
- Use a telegram bot to publish news to a group

Get the chat ID value using the command /chat_id after adding the bot to a group.

Build in DEV:

    just build

Build in RELEASE:

    just release
    just release_musl

Dev:

    golangci-lint run -c .golangci.yml ./...
    betteralign -apply ./...
    nilaway ./...
    deadcode ./...

    gofumpt -l -w .
