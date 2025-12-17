package mach

import (
	"context"
	_ "embed"
	"encoding/json"

	"github.com/dop251/goja"
	"github.com/machbase/neo-server/v8/api"
	"github.com/machbase/neo-server/v8/api/machcli"
)

//go:embed mach.js
var machJS string

func Module(rt *goja.Runtime, module *goja.Object) {
	// Export native functions to embedded JS module
	m := rt.NewObject()
	m.Set("NewDatabase", NewDatabase)
	rt.Set("_mach", m)

	// Run the embedded JS module code
	rt.Set("module", module)
	_, err := rt.RunString("(()=>{" + machJS + "})()")
	if err != nil {
		panic(err)
	}
	rt.Set("module", goja.Undefined())
}

type Config struct {
	Host            string `json:"host"`
	Port            int    `json:"port"`
	User            string `json:"user"`
	Password        string `json:"password"`
	AlternativeHost string `json:"alternativeHost,omitempty"`
	AlternativePort int    `json:"alternativePort,omitempty"`
}

type Database struct {
	Ctx      context.Context
	Cancel   context.CancelFunc
	cli      *machcli.Database
	user     string
	password string
}

func NewDatabase(data string) (*Database, error) {
	obj := Config{}
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return nil, err
	}
	conf := &machcli.Config{
		Host: obj.Host,
		Port: obj.Port,
	}
	if obj.AlternativeHost != "" {
		conf.AlternativeHost = obj.AlternativeHost
	}
	if obj.AlternativePort != 0 {
		conf.AlternativePort = obj.AlternativePort
	}
	db, err := machcli.NewDatabase(conf)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Database{
		Ctx:      ctx,
		Cancel:   cancel,
		cli:      db,
		user:     obj.User,
		password: obj.Password,
	}, nil
}

func (db *Database) Close() error {
	return db.cli.Close()
}

func (db *Database) Connect() (*machcli.Conn, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	conn, err := db.cli.Connect(ctx, api.WithPassword(db.user, db.password))
	if err != nil {
		return nil, err
	}
	return conn.(*machcli.Conn), nil
}
