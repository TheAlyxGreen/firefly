# Firefly

A Go library for BlueSky/AtProto that simplifies common operations with clean, idiomatic APIs.

## Features

- **Authentication** - Automatic JWT token refresh and session management
- **Post Creation** - Fragment-based post composition with automatic rich text handling
- **Search** - Post and user search with filtering options
- **Real-time Events** - WebSocket firehose streaming with reconnection
- **Threading** - Automatic reply structure management
- **Notifications** - Notification fetching and filtering
- **Content Labels** - Simple content warning system

## Installation

```bash
go get github.com/TheAlyxGreen/firefly
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    
    "github.com/TheAlyxGreen/firefly"
)

func main() {
    // Create client and login
    client, err := firefly.NewDefaultInstance()
    if err != nil {
        log.Fatal(err)
    }

    err = client.Login(context.Background(), "your-username", "your-password")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Logged in as: %s\n", client.Self.Handle)
}
```

## Creating Posts

```go
// Create a post with rich text
post := firefly.NewDraftPost().
    AddText("Hello ").
    AddMention("@alice", "alice.bsky.social").
    AddText(" check out ").
    AddLink("this site", "https://example.com").
    AddHashtag("golang")

// Publish it
result, err := client.PublishDraftPost(post)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Posted: %s\n", result.Uri)
```

### Replying to Posts

```go
// Get a post to reply to
posts, err := client.SearchPosts("golang", 10, nil)
if err != nil {
    log.Fatal(err)
}

// Create a reply (threading handled automatically)
reply := firefly.NewDraftPost().AddText("Great point!")
result, err := client.PostReply(posts[0], reply)
if err != nil {
    log.Fatal(err)
}
```

## Searching

```go
// Simple search
posts, err := client.SearchPosts("golang", 25, nil)
if err != nil {
    log.Fatal(err)
}

// Search with filters
posts, err = client.SearchPosts("climate", 50, &firefly.PostSearch{
    Author:   "scientist.bsky.social",
    Language: "en",
    SortBy:   firefly.SortByTop,
})
if err != nil {
    log.Fatal(err)
}

for _, post := range posts {
    fmt.Printf("%s: %s\n", post.Author.Handle, post.Text)
}
```

## Real-time Firehose

```go
// Stream live events
events, err := client.StreamEvents(ctx, &firefly.FirehoseOptions{
    Collections: []string{"app.bsky.feed.post"},
    BufferSize:  1000,
})
if err != nil {
    log.Fatal(err)
}

for event := range events {
    if event.Type == firefly.EventTypePost {
        fmt.Printf("New post: %s\n", event.Post.Text)
    }
}
```

## Notifications

```go
notifications, err := client.GetNotifications(context.Background(), "", 25)
if err != nil {
    log.Fatal(err)
}

for _, notif := range notifications {
    switch notif.Reason {
    case firefly.NotificationLike:
        fmt.Printf("%s liked your post\n", notif.Author.Handle)
    case firefly.NotificationReply:
        fmt.Printf("%s replied: %s\n", notif.Author.Handle, notif.Record.Text)
    }
}
```

## Error Handling

```go
// Monitor background errors
go func() {
    for err := range client.ErrorChan {
        log.Printf("Background error: %v", err)
    }
}()
```

## Key Concepts

### Fragment-Based Posts

Posts are built from composable fragments:

- `NewText()` - Plain text
- `NewPostMention()` - User mentions (handles DIDs automatically)
- `NewLink()` - External links  
- `NewHashtag()` - Hashtags

Rich text positioning and facet generation is handled automatically.

### Thread Management

The `PostReply()` method automatically handles BlueSky's parent/root thread structure - just pass the post you're replying to.

### Content Labels

Add content warnings with simple strings:

```go
post.SetLabels("nudity", "graphic-media")
```

## Development

```bash
go build
go test ./...
go fmt ./...
```

## License

MIT License

## Contributing

Pull requests and issues welcome.

---

Firefly wraps BlueSky's AT Protocol with simpler Go APIs. It handles the complex parts so you can focus on your application.