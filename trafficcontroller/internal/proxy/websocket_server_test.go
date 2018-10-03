package proxy_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"time"

	loggregator "code.cloudfoundry.org/go-loggregator"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"code.cloudfoundry.org/loggregator/metricemitter/testhelper"
	. "code.cloudfoundry.org/loggregator/trafficcontroller/internal/proxy"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("WebsocketServer", func() {
	Describe("Slow Consumer", func() {
		var (
			metricClient *testhelper.SpyMetricClient
			mockHealth   *mockHealth
		)

		BeforeEach(func() {
			metricClient = testhelper.NewMetricClient()
			mockHealth = newMockHealth()
			em := metricClient.NewCounter("egress")
			s := NewWebSocketServer(time.Millisecond, metricClient, mockHealth)

			req, _ := http.NewRequest("GET", "/some", nil)
			req.RemoteAddr = "some-address"
			req.Header["X-Forwarded-For"] = []string{"192.0.0.1", "192.0.0.2"}

			s.ServeWS(
				context.Background(),
				httptest.NewRecorder(),
				req,
				loggregator.EnvelopeStream(func() []*loggregator_v2.Envelope {
					return []*loggregator_v2.Envelope{
						{
							SourceId: "abc-123",
							Message: &loggregator_v2.Envelope_Log{
								Log: &loggregator_v2.Log{
									Payload: []byte("hello"),
								},
							},
						},
					}
				}),
				em,
			)
		})

		It("increments a counter", func() {
			Eventually(func() uint64 {
				return metricClient.GetDelta("doppler_proxy.slow_consumer")
			}).ShouldNot(BeZero())
		})

		It("emits an event", func() {
			expectedBody := sanitizeWhitespace(`
Remote Address: some-address
X-Forwarded-For: 192.0.0.1, 192.0.0.2
Path: /some

When Loggregator detects a slow connection, that connection is disconnected to
prevent back pressure on the system. This may be due to improperly scaled
nozzles, or slow user connections to Loggregator`)

			Eventually(func() string {
				return sanitizeWhitespace(metricClient.GetEvent("Traffic Controller has disconnected slow consumer"))
			}).Should(Equal(expectedBody))
		})

		It("increments a health counter", func() {
			Eventually(mockHealth.IncInput.Name).Should(Receive(Equal("slowConsumerCount")))
		})
	})
})

func sanitizeWhitespace(s string) string {
	return regexp.MustCompile(`\s`).ReplaceAllString(s, "")
}
