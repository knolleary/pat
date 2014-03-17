package workloads_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/url"

	"github.com/julz/pat/config"
	. "github.com/julz/pat/workloads"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type workloads interface {
	Target() error
	Login() error
	Push() error
	DescribeParameters(config.Config)
}

var _ = Describe("Rest Workloads", func() {
	var (
		client            *dummyClient
		ctx               workloads
		args              []string
		replies           map[string]interface{}
		replyWithLocation map[string]string
	)

	BeforeEach(func() {
		replies = make(map[string]interface{})
		replyWithLocation = make(map[string]string)
		client = &dummyClient{replies, replyWithLocation, make(map[call]interface{})}
		ctx = NewContext(client)
		config := config.NewConfig()
		ctx.DescribeParameters(config)
		config.Parse(args)
		args = []string{"-rest:target", "APISERVER"}

		replies["APISERVER/v2/info"] = TargetResponse{"THELOGINSERVER/PATH"}
	})

	Describe("Pushing an app", func() {
		Context("When the user has not logged in", func() {
			It("Returns an error", func() {
				err := ctx.Push()
				Ω(err).Should(HaveOccurred())
			})
		})

		Context("After logging in", func() {
			BeforeEach(func() {
				replies["THELOGINSERVER/PATH/oauth/token"] = LoginResponse{"blah blah"}

				spaceReply := SpaceResponse{[]Resource{Resource{Metadata{"blah blah"}}}}
				replies["APISERVER/v2/spaces?q=name:dev"] = spaceReply

				replyWithLocation["APISERVER/v2/apps"] = "/THE-APP-URI"
				replies["APISERVER/THE-APP-URI"] = ""
				replies["APISERVER/THE-APP-URI/bits"] = ""

				err := ctx.Target()
				Ω(err).ShouldNot(HaveOccurred())
				err = ctx.Login()
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("Doesn't return an error", func() {
				err := ctx.Push()
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("POSTs a (random) name and the chosen space's guid", func() {
				ctx.Push()
				data := client.ShouldHaveBeenCalledWith("POST", "APISERVER/v2/apps")
				m := mapOf(data)
				Ω(m).Should(HaveKey("name"))
				Ω(m).Should(HaveKey("space_guid"))
				Ω(m["space_guid"]).Should(Equal("blah blah"))
			})

			It("Uploads app bits", func() {
				ctx.Push()
				data := client.ShouldHaveBeenCalledWith("PUT(multipart)", "APISERVER/THE-APP-URI/bits")
				Ω(data).ShouldNot(BeNil())
			})

			It("Starts the app", func() {
				ctx.Push()
				data := mapOf(client.ShouldHaveBeenCalledWith("PUT", "APISERVER/THE-APP-URI"))
				Ω(data["state"]).Should(Equal("STARTED"))
			})

			Context("When the app starts immediately", func() {
				It("Doesn't return any error", func() {
					replies["APISERVER/THE-APP-URI/instances"] = "foo" // return a 200
					err := ctx.Push()
					Ω(err).ShouldNot(HaveOccurred())
				})
			})

			Context("When the app status eventually returns CF-NotStaged", func() {
				PIt("Returns an error", func() {
				})
			})
		})
	})

	Describe("Logging in", func() {

		Context("When the API has been targetted", func() {
			JustBeforeEach(func() {
				ctx.Target()
			})

			It("Can log in to the authorization endpoint", func() {
				ctx.Login()
				client.ShouldHaveBeenCalledWith("POST(uaa)", "THELOGINSERVER/PATH/oauth/token")
			})

			Context("When a username and password are configured", func() {
				BeforeEach(func() {
					args = []string{"-rest:target", "APISERVER", "-rest:space", "thespace", "-rest:username", "foo", "-rest:password", "bar"}
				})

				JustBeforeEach(func() {
					ctx.Login()
				})

				It("sets grant_type password", func() {
					data := client.ShouldHaveBeenCalledWith("POST(uaa)", "THELOGINSERVER/PATH/oauth/token")
					Ω(data.(url.Values)["grant_type"]).Should(Equal([]string{"password"}))
				})

				It("POSTs the username and password", func() {
					data := client.ShouldHaveBeenCalledWith("POST(uaa)", "THELOGINSERVER/PATH/oauth/token")
					Ω(data.(url.Values)["username"]).Should(Equal([]string{"foo"}))
					Ω(data.(url.Values)["password"]).Should(Equal([]string{"bar"}))
				})

				It("sets empty scope", func() {
					data := client.ShouldHaveBeenCalledWith("POST(uaa)", "THELOGINSERVER/PATH/oauth/token")
					Ω(data.(url.Values)["scope"]).Should(Equal([]string{""}))
				})

				Context("And the login is successful", func() {
					BeforeEach(func() {
						replies["THELOGINSERVER/PATH/oauth/token"] = struct {
							AccessToken string `json:"access_token"`
						}{"blah blah"}

						spaceReply := SpaceResponse{[]Resource{Resource{Metadata{"blah blah"}}}}
						replies["APISERVER/v2/spaces?q=name:thespace"] = spaceReply
					})

					It("Does not return an error", func() {
						err := ctx.Login()
						Ω(err).ShouldNot(HaveOccurred())
					})

					Context("But when the space does not exist", func() {
						BeforeEach(func() {
							replies["APISERVER/v2/spaces?q=name:thespace"] = nil
						})

						It("Returns an error", func() {
							err := ctx.Login()
							Ω(err).Should(HaveOccurred())
						})
					})
				})

				Context("And the login is not successful", func() {
					BeforeEach(func() {
						replies["THELOGINSERVER/path/oauth/token"] = nil
					})

					It("Does not return an error", func() {
						err := ctx.Login()
						Ω(err).Should(HaveOccurred())
					})
				})
			})
		})

		Describe("When the API hasn't been targetted yet", func() {
			It("Will return an error", func() {
				err := ctx.Login()
				Ω(err).To(HaveOccured())
			})
		})
	})
})

type dummyClient struct {
	replies           map[string]interface{}
	replyWithLocation map[string]string
	calls             map[call]interface{}
}

type call struct {
	method string
	path   string
}

func (d *dummyClient) ShouldHaveBeenCalledWith(method string, path string) interface{} {
	Ω(d.calls).Should(HaveKey(call{method, path}))
	return d.calls[call{method, path}]
}

func (d *dummyClient) Req(method string, host string, data interface{}, s interface{}) (reply Reply) {
	d.calls[call{method, host}] = data
	if d.replyWithLocation[host] != "" {
		return Reply{201, "Moved", d.replyWithLocation[host]}
	}
	if d.replies[host] == nil {
		return Reply{400, "Some error", ""}
	}
	b, _ := json.Marshal(d.replies[host])
	json.NewDecoder(bytes.NewReader(b)).Decode(s)
	return Reply{200, "Success", ""}
}

func (d *dummyClient) Get(host string, data interface{}, s interface{}) (reply Reply) {
	return d.Req("GET", host, data, s)
}

func (d *dummyClient) MultipartPut(m *multipart.Writer, host string, data interface{}, s interface{}) (reply Reply) {
	return d.Req("PUT(multipart)", host, data, s)
}

func (d *dummyClient) Put(host string, data interface{}, s interface{}) (reply Reply) {
	return d.Req("PUT", host, data, s)
}

func (d *dummyClient) Post(host string, data interface{}, s interface{}) (reply Reply) {
	return d.Req("POST", host, data, s)
}

func (d *dummyClient) PostToUaa(host string, data url.Values, s interface{}) (reply Reply) {
	return d.Req("POST(uaa)", host, data, s)
}

func mapOf(data interface{}) map[string]interface{} {
	d, _ := json.Marshal(data)
	m := make(map[string]interface{})
	json.Unmarshal(d, &m)
	return m
}
