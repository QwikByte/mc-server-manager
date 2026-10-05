package rcon

import (
	"context"
	"sync"
)

// Consoles keeps one connection to the console of each game server, as servers log every new
// one, and runs the commands of a server one after the other, in the order they come.
type Consoles struct {
	mu       sync.Mutex
	consoles map[string]*console
}

type console struct {
	turn chan struct{} // holds a value while a command runs
	conn *conn
	run  string
}

func NewConsoles() *Consoles { return &Consoles{consoles: map[string]*console{}} }

// Target returns the address and password of the console of a server.
type Target func(ctx context.Context) (addr, password string, err error)

// Command runs a command on the console of the server id and returns its answer. run tells
// the current run of the server from earlier ones, e.g. by its address and start time: a
// connection to an earlier run is replaced. target is asked when a connection is needed.
func (c *Consoles) Command(ctx context.Context, id, run string, target Target, command string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	con := c.console(id)
	select {
	case con.turn <- struct{}{}:
		defer func() { <-con.turn }()
	case <-ctx.Done():
		return "", ctx.Err()
	}
	if con.run != run {
		con.close()
	}
	if con.conn == nil {
		addr, password, err := target(ctx)
		if err == nil {
			con.conn, err = dial(ctx, addr, password)
		}
		if err != nil {
			return "", err
		}
		con.run = run
	}
	answer, err := con.conn.call(ctx, typeCommand, command)
	if err != nil {
		con.close() // its answer may still come, which the next command mustn't read
	}
	return answer, err
}

// Close closes the connection to the console of a server, e.g. one that stopped.
func (c *Consoles) Close(id string) {
	c.mu.Lock()
	con, ok := c.consoles[id]
	c.mu.Unlock()
	if ok {
		con.turn <- struct{}{}
		con.close()
		<-con.turn
	}
}

func (c *Consoles) console(id string) *console {
	c.mu.Lock()
	defer c.mu.Unlock()
	con, ok := c.consoles[id]
	if !ok {
		con = &console{turn: make(chan struct{}, 1)}
		c.consoles[id] = con
	}
	return con
}

func (con *console) close() {
	if con.conn != nil {
		_ = con.conn.Close()
		con.conn = nil
	}
}
