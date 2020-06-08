package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	"firebase.google.com/go/messaging"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	const appEngineCronHeader = "X-Appengine-Cron"
	http.HandleFunc("/_job", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(appEngineCronHeader) != "true" {
			fmt.Fprint(w, "error")
			return
		}
		ctx := r.Context()

		app, err := firebase.NewApp(ctx, nil)
		if err != nil {
			log.Print(err)
			fmt.Fprint(w, "error")
			return
		}

		store, err := app.Firestore(ctx)
		if err != nil {
			log.Print(err)
			fmt.Fprint(w, "error")
			return
		}

		now := time.Now().UTC()

		err = store.RunTransaction(ctx, func(ctx context.Context, t *firestore.Transaction) error {
			doc := store.Collection("Lock").Doc("Notification")
			snap, err := doc.Get(ctx)
			if err == nil {
				date, ok := snap.Data()["Date"]
				if ok {
					d, ok := date.(time.Time)
					if ok && now.Hour() == d.UTC().Hour() {
						return errors.New("failed to take lock")
					}
				}
			} else if status.Code(err) != codes.NotFound {
				return err
			}

			_, err = doc.Set(ctx, map[string]interface{}{
				"Date": now,
			})
			return err
		})
		if err != nil {
			log.Print(err)
			fmt.Fprint(w, "error")
			return
		}

		cli, err := app.Messaging(ctx)
		if err != nil {
			log.Print(err)
			fmt.Fprint(w, "error")
			return
		}

		jst := time.FixedZone("Asia/Tokyo", 9*60*60)
		msg := &messaging.Message{
			Topic: "sample",
			Notification: &messaging.Notification{
				Title: "now(JST)",
				Body:  now.In(jst).String(),
			},
			Data: map[string]string{
				"data": "hogehogehoge",
			},
		}
		s, err := cli.Send(ctx, msg)
		if err != nil {
			log.Print(err)
			fmt.Fprint(w, "error")
			return
		}

		log.Printf("result: %v", s)
		fmt.Fprint(w, "done")
	})

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
