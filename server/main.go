package main

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go"
	"firebase.google.com/go/messaging"
	"google.golang.org/api/option"
	"google.golang.org/api/transport/cert"
	ghttp "google.golang.org/api/transport/http"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	const appEngineCronHeader = "X-Appengine-Cron"
	jst := time.FixedZone("Asia/Tokyo", 9*60*60)
	http.HandleFunc("/_job", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(appEngineCronHeader) != "true" {
			fmt.Fprint(w, "error")
			return
		}
		ctx := r.Context()

		now := time.Now().UTC()
		err := notificationLock(ctx, now)
		if err != nil {
			log.Print(err)
			fmt.Fprint(w, "error")
			return
		}

		cli, err := createMessaging(ctx)
		if err != nil {
			log.Print(err)
			fmt.Fprint(w, "error")
			return
		}

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

func notificationLock(ctx context.Context, now time.Time) error {
	store, err := firestore.NewClient(ctx, firestore.DetectProjectID)
	if err != nil {
		return err
	}

	return store.RunTransaction(ctx, func(ctx context.Context, t *firestore.Transaction) error {
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
}

func createMessaging(ctx context.Context) (*messaging.Client, error) {
	opts, err := createClientOptions(ctx)
	if err != nil {
		return nil, err
	}

	app, err := firebase.NewApp(ctx, nil, opts...)
	if err != nil {
		return nil, err
	}

	cli, err := app.Messaging(ctx)
	if err != nil {
		return nil, err
	}

	return cli, err
}

func createClientOptions(ctx context.Context) ([]option.ClientOption, error) {
	certSource, err := cert.DefaultSource()
	if err != nil {
		return nil, err
	}
	var baseTrans http.RoundTripper
	if certSource != nil {
		baseTrans = &http.Transport{
			TLSClientConfig: &tls.Config{
				GetClientCertificate: certSource,
			},
		}
	} else {
		baseTrans = http.DefaultTransport
	}
	var firebaseScopes = []string{
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/datastore",
		"https://www.googleapis.com/auth/devstorage.full_control",
		"https://www.googleapis.com/auth/firebase",
		"https://www.googleapis.com/auth/identitytoolkit",
		"https://www.googleapis.com/auth/userinfo.email",
	}
	o := []option.ClientOption{option.WithScopes(firebaseScopes...)}
	trans, err := ghttp.NewTransport(ctx, baseTrans, o...)
	if err != nil {
		return nil, err
	}
	hc := &http.Client{
		Transport: &rt{
			base: trans,
		},
	}
	opt := option.WithHTTPClient(hc)
	return []option.ClientOption{opt}, nil
}

type rt struct {
	base http.RoundTripper
}

func (t *rt) RoundTrip(r *http.Request) (*http.Response, error) {
	dump, _ := httputil.DumpRequestOut(r, true)
	log.Printf("req:%s", dump)

	resp, err := t.base.RoundTrip(r)
	dump, _ = httputil.DumpResponse(resp, true)
	log.Printf("resp:%s", dump)

	return resp, err
}
