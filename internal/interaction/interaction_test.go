package interaction

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewService(client, nil, Options{})
}

func TestToggleLike(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()

	liked, count, err := s.ToggleLike(ctx, 1, 100)
	if err != nil || !liked || count != 1 {
		t.Fatalf("1st toggle: want liked=true count=1, got liked=%v count=%d err=%v", liked, count, err)
	}
	liked, count, err = s.ToggleLike(ctx, 1, 100)
	if err != nil || liked || count != 0 {
		t.Fatalf("2nd toggle: want liked=false count=0, got liked=%v count=%d err=%v", liked, count, err)
	}
}

func TestToggleFavorite(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()

	fav, count, err := s.ToggleFavorite(ctx, 1, 100)
	if err != nil || !fav || count != 1 {
		t.Fatalf("1st toggle: want favorited=true count=1, got %v %d %v", fav, count, err)
	}
	fav, count, err = s.ToggleFavorite(ctx, 1, 100)
	if err != nil || fav || count != 0 {
		t.Fatalf("2nd toggle: want favorited=false count=0, got %v %d %v", fav, count, err)
	}
}

func TestRecordPlay(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()

	for i := int64(1); i <= 3; i++ {
		count, err := s.RecordPlay(ctx, 1)
		if err != nil || count != i {
			t.Fatalf("play #%d: want count=%d, got %d err=%v", i, i, count, err)
		}
	}
}

func TestCountsAndStatus(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()

	_, _, _ = s.ToggleLike(ctx, 1, 100)
	_, _, _ = s.ToggleFavorite(ctx, 1, 100)
	_, _ = s.RecordPlay(ctx, 1)
	_, _ = s.RecordPlay(ctx, 1)

	counts, err := s.Counts(ctx, []int64{1})
	if err != nil {
		t.Fatal(err)
	}
	c := counts[1]
	if c.LikeCount != 1 || c.FavoriteCount != 1 || c.PlayCount != 2 {
		t.Fatalf("want like=1 fav=1 play=2, got %+v", c)
	}

	liked, _ := s.IsLiked(ctx, 1, 100)
	fav, _ := s.IsFavorited(ctx, 1, 100)
	if !liked || !fav {
		t.Fatalf("want liked && favorited, got liked=%v fav=%v", liked, fav)
	}
}

func TestHotLeaderboard(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()

	_, _ = s.RecordPlay(ctx, 1)
	_, _ = s.RecordPlay(ctx, 1)
	_, _ = s.RecordPlay(ctx, 1)
	_, _ = s.RecordPlay(ctx, 2)

	ids, err := s.Hot(ctx, "play", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
		t.Fatalf("want [1 2] (desc play), got %v", ids)
	}

	_, _, _ = s.ToggleLike(ctx, 2, 100)
	ids, err = s.Hot(ctx, "like", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) == 0 || ids[0] != 2 {
		t.Fatalf("want like leaderboard top=2, got %v", ids)
	}
}

func TestPopEvents(t *testing.T) {
	s := newTestService(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if err := s.rdb.RPush(ctx, pendingKey, marshalEvent(Event{ID: int64(i + 1), Type: EventPlay, VideoID: 1})).Err(); err != nil {
			t.Fatal(err)
		}
	}
	events, err := s.popEvents(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("want 3 popped, got %d", len(events))
	}
	if n, _ := s.rdb.LLen(ctx, pendingKey).Result(); n != 2 {
		t.Fatalf("want 2 remaining, got %d", n)
	}
}
