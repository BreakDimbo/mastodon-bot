package bot

import (
	gomastodon "bot/go-mastodon"
	"bot/intelbot/const"
	"bot/intelbot/elastics"
	"context"
	"fmt"
	"time"

	"github.com/microcosm-cc/bluemonday"
)

type indexStatus struct {
	ID              string    `json:"id"`
	CreatedAt       time.Time `json:"created_at"`
	AccountId       string    `json:"account_id"`
	Content         string    `json:"content"`
	ReblogsCount    int64     `json:"reblogs_count"`
	FavouritesCount int64     `json:"favourites_count"`
	Sensitive       bool      `json:"sensitive"`
	Scope           string    `json:"scope"` //curl -XPUT "http://localhost:9200/status/_mapping/status" -H 'Content-Type: application/json' -d '{"properties": {"scope": {"type": "keyword"}}}'
}

func HandleUpdate(e *gomastodon.UpdateEvent, scope string) {
	polished := filter(e.Status.Content)
	indexS := &indexStatus{
		ID:              string(e.Status.ID),
		CreatedAt:       e.Status.CreatedAt,
		AccountId:       string(e.Status.Account.ID),
		Content:         polished,
		ReblogsCount:    e.Status.ReblogsCount,
		FavouritesCount: e.Status.FavouritesCount,
		Sensitive:       e.Status.Sensitive,
	}

	var index string
	switch scope {
	case con.ScopeTypeLocal:
		index = "local"
	case con.ScopeTypePublic:
		index = "status"
	}

	ctx := context.Background()
	p, err := elastics.Client.Index().
		Index(index).
		Type("status").
		Id(indexS.ID).
		BodyJson(indexS).
		Do(ctx)
	if err != nil {
		fmt.Printf("[ERROR] update to es error: %s/n", err)
		return
	}
	fmt.Printf("Indexed status %s to index %s, type %s, scope %s\n", p.Id, p.Index, p.Type, scope)
}

func HandleDelete(e *gomastodon.DeleteEvent, scope string) {
	var index string
	switch scope {
	case con.ScopeTypeLocal:
		index = "local"
	case con.ScopeTypePublic:
		index = "status"
	}

	ctx := context.Background()
	_, err := elastics.Client.Delete().Index(index).Type("status").Id(e.ID).Do(ctx)
	if err != nil {
		fmt.Printf("[ERROR] delete %s from es error: %s\n", e.ID, err)
		return
	}
	fmt.Printf("delete from es ok with id: %s\n", e.ID)
}

func HandleNotification(e *gomastodon.NotificationEvent) {
	switch e.Notification.Type {
	case "follow":
		ctx := context.Background()
		accountId := e.Notification.Account.ID
		_, err := botClient.Normal.AccountFollow(ctx, accountId)
		if err != nil {
			fmt.Printf("[Error] follow account error: %s", err)
		}
	}
}

func CleanUnfollower() {
	fmt.Printf("Start cleaning unfollowers\n")
	ctx := context.Background()
	pg := &gomastodon.Pagination{Limit: 80}
	followerM := make(map[gomastodon.ID]bool)
	followingM := make(map[gomastodon.ID]bool)

	ca, err := botClient.Normal.GetAccountCurrentUser(ctx)
	checkErr(err)

	followers, err := botClient.Normal.GetAccountFollowers(ctx, ca.ID, pg)
	checkErr(err)
	for _, v := range followers {
		followerM[v.ID] = true
	}

	followings, err := botClient.Normal.GetAccountFollowing(ctx, ca.ID, pg)
	checkErr(err)
	for _, v := range followings {
		followingM[v.ID] = true
	}

	for k, _ := range followingM {
		if _, ok := followerM[k]; !ok {
			_, err := botClient.Normal.AccountUnfollow(ctx, k)
			checkErr(err)
		}
	}
}

func filter(raw string) (polished string) {
	p := bluemonday.StrictPolicy()
	polished = p.Sanitize(raw)
	return
}

func checkErr(err error) {
	if err != nil {
		fmt.Printf("[ERROR] get error: %s\n", err)
	}
}