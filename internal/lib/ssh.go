package lib

import (
	"fmt"
	"io"
	"net"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
)

// MakeSSHSession starts a session for running a command. Returns the session and
// the stderr pipe.
func MakeSSHSession(addr string, port int, user string, pemBytes []byte) (*ssh.Session, io.Reader, error) {
	addrPort := fmt.Sprintf("%s:%d", addr, port)
	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		return nil, nil, errors.Wrap(err, "parsing private key")
	}

	sshConfig := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: make it secure
	}
	client, err := ssh.Dial("tcp", addrPort, sshConfig)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "dialing to %s", addrPort)
	}

	toCloseClient := true
	defer func() {
		if toCloseClient {
			client.Close()
		}
	}()
	session, err := client.NewSession()
	if err != nil {
		return nil, nil, errors.Wrap(err, "starting session")
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return nil, nil, errors.Wrap(err, "creating stderr pipe")
	}

	toCloseClient = false
	return session, stderr, nil
}

// SSHRespondingServer is a mock sshd server that expects a command and writes
// back payload.
// Some parts taken from https://gist.github.com/jpillora/b480fde82bff51a06238
type SSHRespondingServer struct {
	port            int
	hostKey         []byte
	expectedCommand string
	responseReader  io.Reader
	logger          *zap.SugaredLogger
	stop            chan struct{}
}

func NewSSHRespondingServer(
	port int,
	hostKey []byte,
	expectedCommand string,
	responseReader io.Reader,
	logger *zap.SugaredLogger,
) *SSHRespondingServer {
	return &SSHRespondingServer{
		port:            port,
		hostKey:         hostKey,
		expectedCommand: expectedCommand,
		responseReader:  responseReader,
		logger:          logger,
		stop:            make(chan struct{}),
	}
}

func (rs *SSHRespondingServer) StartAsync() {
	go rs.runServer()
}

func (rs *SSHRespondingServer) Stop() {
	rs.stop <- struct{}{}
}

func (rs *SSHRespondingServer) runServer() {
	config := &ssh.ServerConfig{NoClientAuth: true}
	hostKey, err := ssh.ParsePrivateKey(rs.hostKey)
	if err != nil {
		rs.logger.Errorf("failed parsing host key: %v", err)
	}
	config.AddHostKey(hostKey)
	addrPort := fmt.Sprintf("%s:%d", "localhost", rs.port)
	rs.logger.Infof("listening on %s", addrPort)
	listener, err := net.Listen("tcp", addrPort)
	if err != nil {
		rs.logger.Errorf("failed listening: %s", err)
	}

	acceptChan := make(chan net.Conn)
	waitForAccept := func() {
		tcpConn, err := listener.Accept()
		if err != nil {
			rs.logger.Errorf("failed to accept connection: %s", err)
		}
		acceptChan <- tcpConn
	}
	for {
		go waitForAccept()
		var conn net.Conn
		select {
		case c := <-acceptChan:
			conn = c
		case <-rs.stop:
			return
		}
		if conn == nil {
			continue
		}
		// Before use, a handshake must be performed on the incoming net.Conn.
		sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
		if err != nil {
			rs.logger.Errorf("failed to handshake: %s", err)
			continue
		}
		rs.logger.Infof("New SSH connection from %s (%s)", sshConn.RemoteAddr(), sshConn.ClientVersion())
		go ssh.DiscardRequests(reqs)
		for newChannel := range chans {
			go rs.handleChannel(newChannel)
		}
	}
}

func (rs *SSHRespondingServer) handleChannel(newChannel ssh.NewChannel) {
	if t := newChannel.ChannelType(); t != "session" {
		newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %s", t))
		return
	}

	// At this point, we have the opportunity to reject the client's
	// request for another logical connection
	connection, requests, err := newChannel.Accept()
	if err != nil {
		rs.logger.Errorf("Could not accept channel (%v)", err)
		return
	}

	// Sessions have out-of-band requests such as "shell", "pty-req" and "env"
	go func() {
		for req := range requests {
			switch req.Type {
			case "exec":
				type execMsg struct {
					Command string
				}
				var msg execMsg
				err := ssh.Unmarshal(req.Payload, &msg)
				if msg.Command == rs.expectedCommand {
					go func() {
						io.Copy(connection, rs.responseReader)
						connection.Close()
						rs.logger.Info("Session closed")
					}()
				} else {
					rs.logger.Warnf("Received unexpected command. Expected [%s], Got [%s]", rs.expectedCommand, msg.Command)
				}

				req.Reply(err == nil, nil)
			case "shell":
				// We only accept the default shell
				// (i.e. no command in the Payload)
				if len(req.Payload) == 0 {
					req.Reply(true, nil)
				}
			case "pty-req":
				// Responding true (OK) here will let the client
				// know we have a pty ready for input
				req.Reply(true, nil)
			case "window-change":
			}
		}
	}()
}
