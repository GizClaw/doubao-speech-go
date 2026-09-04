# Podcast generation

The Podcast service wraps `wss://openspeech.bytedance.com/api/v3/sami/podcasttts`.
It exposes the provider's round boundaries so callers can persist each complete
round and resume an interrupted task with `PodcastRetryInfo`.

```go
client := doubaospeech.NewClient(
    appID,
    doubaospeech.WithAccessKey(accessKey),
)

session, err := client.Podcast.OpenSession(ctx, &doubaospeech.PodcastRequest{
    InputID:    "episode-001",
    PromptText: "Two hosts explain why the moon has phases.",
})
if err != nil {
    return err
}
defer session.Close()

for {
    event, err := session.RecvEvent(ctx)
    if err != nil {
        return err
    }
    switch event.Type {
    case doubaospeech.PodcastEventRoundStarted:
        // Start a temporary file for event.RoundID.
    case doubaospeech.PodcastEventAudio:
        // Append event.Audio to the current round.
    case doubaospeech.PodcastEventRoundFinished:
        // Atomically commit the round, then persist event.RoundID.
    case doubaospeech.PodcastEventSessionFinished:
        return nil
    }
}
```

Authentication headers are configured with `WithAccessKey` and, when needed,
`WithAppKey`. When `WithAppKey` is omitted, the service uses the standard
Podcast app-key value. `WithResourceID` can override `ResourcePodcast`.

After an interrupted run, open a new session and provide the original task ID
plus the last atomically persisted round:

```go
RetryInfo: &doubaospeech.PodcastRetryInfo{
    TaskID:              originalTaskID,
    LastFinishedRoundID: lastRoundID,
},
```
