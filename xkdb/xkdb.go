package xxdb

import (
	"bufio"
	"bytes"
	"net"

	pb "github.com/X-Company/geocoding/proto/xxdb"
	"google.golang.org/protobuf/proto"
)

const DELIM string = "==DELIM=="

func splitDELIM(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.Index(data, []byte(DELIM)); i >= 0 {
		// We have a full DELIM-terminated line.
		return i + len(DELIM), data[:i], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

type Client struct {
	scanner *bufio.Scanner
	conn    net.Conn
}

func NewClient(conn net.Conn, user string, pass string) (*Client, error) {
	scanner := bufio.NewScanner(conn)
	scanner.Split(splitDELIM)

	client := &Client{scanner: scanner, conn: conn}

	auth := &pb.Auth{User: user, Pass: pass}
	if resp, err := client.Exec(auth); err != nil {
		return nil, err
	} else if resp.Status != pb.Response_OK {
		return nil, err
	}

	return client, nil
}

func (c *Client) Exec(query proto.Message) (*pb.Response, error) {
	if out, err := proto.Marshal(query); err != nil {
		return nil, err
	} else if _, err := c.conn.Write(append(out, []byte(DELIM)...)); err != nil {
		return nil, err
	}

	c.scanner.Scan()
	if err := c.scanner.Err(); err != nil {
		return nil, err
	}

	resp := &pb.Response{}
	if err := proto.Unmarshal(c.scanner.Bytes(), resp); err != nil {
		return nil, err
	}

	return resp, nil
}
