package doppler_test

import (
	"net"
	"time"

	"code.cloudfoundry.org/loggregator/doppler/app/config"
	"code.cloudfoundry.org/loggregator/plumbing"

	"github.com/cloudfoundry/dropsonde/signature"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var _ = Describe("GRPC Streaming Logs", func() {
	Context("with a subscription established", func() {
		var (
			conf         *config.Config
			in           net.Conn
			out          *grpc.ClientConn
			subscription plumbing.Doppler_SubscribeClient
		)

		JustBeforeEach(func() {
			conf = fetchDopplerConfig("fixtures/doppler.json")
			in = connectToDoppler()
			out, subscription = connectToSubscription(
				conf,
				plumbing.SubscriptionRequest{
					ShardID: "foo",
					Filter: &plumbing.Filter{
						AppID: "test-app",
					},
				},
			)

			primePump(in)
			waitForPrimer(subscription)
		})

		AfterEach(func() {
			in.Close()
			out.Close()
		})

		It("responds to a subscription request", func() {
			logMessage := buildLogMessage()
			signedLog := signature.SignMessage(logMessage, []byte("secret"))

			_, err := in.Write(signedLog)
			Expect(err).ToNot(HaveOccurred())

			f := func() []byte {
				msg, _ := subscription.Recv()
				if msg == nil {
					return nil
				}

				return msg.Payload
			}
			Eventually(f).Should(Equal(logMessage))
		})
	})
})

func primePump(conn net.Conn) {
	logMessage := buildLogMessage()
	signedLog := signature.SignMessage(logMessage, []byte("secret"))

	go func() {
		for i := 0; i < 20; i++ {
			if _, err := conn.Write(signedLog); err != nil {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
}

func waitForPrimer(subscription plumbing.Doppler_SubscribeClient) {
	_, err := subscription.Recv()
	Expect(err).ToNot(HaveOccurred())
}

func connectToDoppler() net.Conn {
	in, err := net.Dial("udp", localIPAddress+":8765")
	Expect(err).ToNot(HaveOccurred())
	return in
}

func connectToSubscription(conf *config.Config, req plumbing.SubscriptionRequest) (*grpc.ClientConn, plumbing.Doppler_SubscribeClient) {
	conn, client := connectToGRPC(conf)

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	subscription, err := client.Subscribe(ctx, &req)
	Expect(err).ToNot(HaveOccurred())

	return conn, subscription
}
