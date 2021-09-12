package receiver

import (
	"encoding/json"
	"errors"
	"reflect"

	"github.com/Postcord/rest"
	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

type EventHandler interface{}

// Receiver is a client to interface with the Chator NATS Discord interface
type Receiver struct {
	conn        *nats.Conn
	session     *rest.Client
	handlers    map[string][]EventHandler
	log         *logrus.Logger
	serviceName string
}

// Config contains the configuration options for the Receiver
type Config struct {
	NatsAddr    string
	Token       string
	Logger      *logrus.Logger
	Client      *rest.Client
	ServiceName string
}

// NewReceiver Creates a new Discord NATS receiver
func New(conf *Config) (*Receiver, error) {
	if conf.Token == "" && conf.Client == nil {
		return nil, errors.New("no valid token provided")
	}

	if conf.NatsAddr == "" {
		conf.NatsAddr = nats.DefaultURL
	}

	if conf.Logger == nil {
		conf.Logger = logrus.New()
		conf.Logger.SetLevel(logrus.DebugLevel)
		conf.Logger.SetFormatter(&logrus.TextFormatter{
			ForceColors:      true,
			DisableTimestamp: false,
			FullTimestamp:    true,
			TimestampFormat:  "",
		})
	}

	var err error

	r := &Receiver{
		handlers: make(map[string][]EventHandler),
	}
	r.conn, err = nats.Connect(conf.NatsAddr)
	if err != nil {
		return nil, err
	}

	r.log = conf.Logger

	// TODO: Add redis cache interface
	if conf.Client == nil {
		r.session = rest.New(&rest.Config{
			Authorization: "Bot " + conf.Token,
			Ratelimiter: rest.NewMemoryRatelimiter(&rest.MemoryConf{
				MaxRetries: 3,
			}),
			UserAgent: "Postcord/1.0",
		})
		if err != nil {
			return nil, err
		}
	} else {
		r.session = conf.Client
	}

	// r.session = disgord.New(disgord.Config{
	// 	BotToken: conf.Token,
	// 	Logger:   conf.Logger,
	// })

	return r, nil
}

func (r *Receiver) Start() {
	for k := range r.handlers {
		r.log.Infof("Subscribing to %s", k)
		if r.serviceName != "" {
			r.conn.QueueSubscribe(k, r.serviceName, r.listener)
		} else {
			r.conn.Subscribe(k, r.listener)
		}
	}
}

func (r *Receiver) Close() {
	r.conn.Close()
}

// On registers an event listener
func (r *Receiver) On(sub string, handler EventHandler) {
	if _, ok := r.handlers[sub]; ok {
		r.handlers[sub] = append(r.handlers[sub], handler)
	} else {
		r.handlers[sub] = []EventHandler{handler}
	}
}

func (r *Receiver) listener(m *nats.Msg) {
	defer func() {
		if rec := recover(); rec != nil {
			r.log.Errorf("panic while calling handler for %s", m.Subject)
		}
	}()
	r.log.Debugf("Event received: %s", m.Subject)
	handlers := r.handlers[m.Subject]
	for _, h := range handlers {
		x := reflect.TypeOf(h)

		numIn := x.NumIn()   //Count inbound parameters
		numOut := x.NumOut() //Count outbounding parameters

		if numIn != 2 || numOut != 0 {
			r.log.Warn("Invalid function signature for event ", m.Subject)
			return
		}

		// This is the object to deserialize into
		inType := x.In(1)
		typePtr := reflect.New(inType.Elem())

		obj := typePtr.Interface()

		err := json.Unmarshal(m.Data, obj)
		if err != nil {
			r.log.Warn("Failed to unmarshal ", m.Subject, " into ", obj, ": ", err.Error())
			return
		}

		f := reflect.ValueOf(h)
		f.Call([]reflect.Value{reflect.ValueOf(r.session), typePtr})
	}
}
