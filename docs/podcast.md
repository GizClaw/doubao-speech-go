# Podcast generation

The Podcast service wraps `wss://openspeech.bytedance.com/api/v3/sami/podcasttts`.
It exposes the provider's round boundaries so callers can persist each complete
round and resume an interrupted task with `PodcastRetryInfo`.

```go
client := doubaospeech.NewClient(
    appID,
    doubaospeech.WithAPIKey(apiKey),
)

session, err := client.Podcast.OpenSession(ctx, &doubaospeech.PodcastRequest{
    InputID:    "episode-001",
    PromptText: "Two hosts explain why the moon has phases.",
	SpeakerInfo: &doubaospeech.PodcastSpeakerInfo{
		Speakers: []doubaospeech.PodcastSpeakerID{
			doubaospeech.PodcastSpeakerMizai,
			doubaospeech.PodcastSpeakerDayi,
		},
	},
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

`PodcastSpeakerInfo.Speakers` accepts exactly two voice IDs. PodcastTTS has
dedicated paired voices and also accepts compatible TTS 1.0, TTS 2.0, and ICL
voice-clone IDs. Mix and multi-emotion voices are not supported.

Some TTS 2.0 and ICL 2.0 voices require an explicit model selection. Use the
typed `SpeakerAdditions` collection; callers never need to construct the
provider's dynamic-key JSON object:

```go
SpeakerInfo: &doubaospeech.PodcastSpeakerInfo{
	Speakers: []doubaospeech.PodcastSpeakerID{
		doubaospeech.PodcastSpeakerMizai,
		"S_your_cloned_voice",
	},
	SpeakerAdditions: doubaospeech.PodcastSpeakerAdditions{
		{
			Speaker: "S_your_cloned_voice",
			Model:   doubaospeech.PodcastSpeakerModelSeedTTS20Standard,
		},
	},
},
```

For an already-authored script, set `Action` to
`PodcastActionFromScript` and provide typed `PodcastScriptLine` values in
`NLPTexts`. Source mode accepts either `InputText` or `InputInfo.InputURL`.

Current authentication uses `WithAPIKey`. Legacy accounts can instead configure
`WithAccessKey` and, when needed, `WithAppKey`; when the app key is omitted, the
service uses the standard Podcast app-key value. `WithResourceID` can override
`ResourcePodcast`.

After an interrupted run, open a new session and provide the original task ID
plus the last atomically persisted round:

```go
RetryInfo: &doubaospeech.PodcastRetryInfo{
    TaskID:              originalTaskID,
    LastFinishedRoundID: lastRoundID,
},
```
