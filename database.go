package main

import (
	"errors"
	r "gopkg.in/gorethink/gorethink.v3"
	"os"
)

func InitConnectionPool() {
	var err error

	dbUrl := os.Getenv("DB")
	if dbUrl == "" {
		log.Fatal("DB env variable not specified")
	}

	dbPass := os.Getenv("DBPASS")
	if dbUrl == "" {
		log.Fatal("DBPASS env variable not specified")
	}

	session, err = r.Connect(r.ConnectOpts{
		Address:    dbUrl,
		InitialCap: 10,
		MaxOpen:    10,
		Database:   "OverStats",
		Password:   dbPass,
	})
	if err != nil {
		log.Fatal(err)
	}

	res, err := r.Table("users").Filter(func(user r.Term) r.Term {
		return user.Field("id").Match("^tg")
	}).Changes().Run(session)
	if err != nil {
		log.Fatal(err)
	}

	var change Change
	for res.Next(&change) {
		SessionReport(change)
	}
}

func GetUser(id string) (User, error) {
	res, err := r.Table("users").Get(id).Run(session)
	if err != nil {
		return User{}, err
	}

	var user User
	err = res.One(&user)
	if err == r.ErrEmptyResult {
		return User{}, errors.New("db: row not found")
	}
	if err != nil {
		return User{}, err
	}

	defer res.Close()
	return user, nil
}

func GetRatingTop(platform string, limit int, chat int64) ([]User, error) {
	var (
		res *r.Cursor
		err error
	)

	query := r.Table("users").OrderBy(r.OrderByOpts{Index: r.Desc("rating")})
	if platform == "console" {
		query = query.Filter(r.Row.Field("region").Eq("psn").Or(r.Row.Field("region").Eq("xbl")))
	} else {
		query = query.Filter(r.Row.Field("region").Ne("psn").And(r.Row.Field("region").Ne("xbl")))
	}
	if chat != 0 {
		query = query.Filter(r.Row.Field("chat").Eq(chat))
	}

	res, err = query.Limit(limit).Run(session)

	if err != nil {
		return []User{}, err
	}

	var top []User
	err = res.All(&top)
	if err != nil {
		return []User{}, err
	}

	defer res.Close()
	return top, nil
}

func GetRatingPlace(id string) (Top, error) {
	res, err := r.Do(
		r.Table("users").OrderBy(r.OrderByOpts{Index: r.Desc("rating")}).OffsetsOf(r.Row.Field("id").Eq(id)).Nth(0),
		r.Table("users").Count(),
		func(place r.Term, count r.Term) r.Term {
			return r.Expr(
				map[string]interface{}{
					"place": place.Add(1),
					"rank":  place.Div(count).Mul(100),
				},
			)
		},
	).Run(session)

	var top Top
	err = res.One(&top)
	if err != nil {
		log.Warn(err)
		return Top{}, err
	}

	return top, nil
}

func GetRank(id string, index r.Term) (Top, error) {
	res, err := r.Do(
		r.Table("users").OrderBy(r.Desc(index)).OffsetsOf(r.Row.Field("id").Eq(id)).Nth(0),
		r.Table("users").Count(index.Ne(0)),
		func(place r.Term, count r.Term) r.Term {
			return r.Expr(
				map[string]interface{}{
					"place": place.Add(1),
					"rank":  place.Div(count).Mul(100),
				},
			)
		},
	).Run(session)

	var top Top
	err = res.One(&top)
	if err != nil {
		log.Warn(err)
		return Top{}, err
	}

	return top, nil
}

func InsertUser(user User) (r.WriteResponse, error) {
	newDoc := map[string]interface{}{
		"id":      user.Id,
		"profile": user.Profile,
		"nick":    user.Nick,
		"region":  user.Region,
		"date":    r.Now(),
	}

	res, err := r.Table("users").Insert(newDoc, r.InsertOpts{
		Conflict: "replace",
	}).RunWrite(session)
	if err != nil {
		return r.WriteResponse{}, err
	}

	return res, nil
}

func UpdateUser(user User) (r.WriteResponse, error) {
	newDoc := map[string]interface{}{
		"id":   user.Id,
		"chat": user.Chat,
	}

	res, err := r.Table("users").Get(user.Id).Update(newDoc).RunWrite(session)
	if err != nil {
		return r.WriteResponse{}, err
	}

	return res, nil
}
