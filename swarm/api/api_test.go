
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2016 Go Ethereum作者
//此文件是Go以太坊库的一部分。
//
//Go-Ethereum库是免费软件：您可以重新分发它和/或修改
//根据GNU发布的较低通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊图书馆的发行目的是希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU较低的通用公共许可证，了解更多详细信息。
//
//你应该收到一份GNU较低级别的公共许可证副本
//以及Go以太坊图书馆。如果没有，请参见<http://www.gnu.org/licenses/>。

package api

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/swarm/sctx"
	"github.com/ethereum/go-ethereum/swarm/storage"
)

func init() {
	loglevel := flag.Int("loglevel", 2, "loglevel")
	flag.Parse()
	log.Root().SetHandler(log.CallerFileHandler(log.LvlFilterHandler(log.Lvl(*loglevel), log.StreamHandler(os.Stderr, log.TerminalFormat(true)))))
}

func testAPI(t *testing.T, f func(*API, bool)) {
	datadir, err := ioutil.TempDir("", "bzz-test")
	if err != nil {
		t.Fatalf("unable to create temp dir: %v", err)
	}
	defer os.RemoveAll(datadir)
	fileStore, err := storage.NewLocalFileStore(datadir, make([]byte, 32))
	if err != nil {
		return
	}
	api := NewAPI(fileStore, nil, nil, nil)
	f(api, false)
	f(api, true)
}

type testResponse struct {
	reader storage.LazySectionReader
	*Response
}

func checkResponse(t *testing.T, resp *testResponse, exp *Response) {

	if resp.MimeType != exp.MimeType {
		t.Errorf("incorrect mimeType. expected '%s', got '%s'", exp.MimeType, resp.MimeType)
	}
	if resp.Status != exp.Status {
		t.Errorf("incorrect status. expected '%d', got '%d'", exp.Status, resp.Status)
	}
	if resp.Size != exp.Size {
		t.Errorf("incorrect size. expected '%d', got '%d'", exp.Size, resp.Size)
	}
	if resp.reader != nil {
		content := make([]byte, resp.Size)
		read, _ := resp.reader.Read(content)
		if int64(read) != exp.Size {
			t.Errorf("incorrect content length. expected '%d...', got '%d...'", read, exp.Size)
		}
		resp.Content = string(content)
	}
	if resp.Content != exp.Content {
//如果！bytes.equal（resp.content，exp.content）
		t.Errorf("incorrect content. expected '%s...', got '%s...'", string(exp.Content), string(resp.Content))
	}
}

//func expresponse（content[]byte，mimetype string，status int）*响应
func expResponse(content string, mimeType string, status int) *Response {
	log.Trace(fmt.Sprintf("expected content (%v): %v ", len(content), content))
	return &Response{mimeType, status, int64(len(content)), content}
}

func testGet(t *testing.T, api *API, bzzhash, path string) *testResponse {
	addr := storage.Address(common.Hex2Bytes(bzzhash))
	reader, mimeType, status, _, err := api.Get(context.TODO(), NOOPDecrypt, addr, path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	quitC := make(chan bool)
	size, err := reader.Size(context.TODO(), quitC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	log.Trace(fmt.Sprintf("reader size: %v ", size))
	s := make([]byte, size)
	_, err = reader.Read(s)
	if err != io.EOF {
		t.Fatalf("unexpected error: %v", err)
	}
	reader.Seek(0, 0)
	return &testResponse{reader, &Response{mimeType, status, size, string(s)}}
//返回&testreresponse reader，&response mimetype，status，reader.size（），nil
}

func TestApiPut(t *testing.T) {
	testAPI(t, func(api *API, toEncrypt bool) {
		content := "hello"
		exp := expResponse(content, "text/plain", 0)
		ctx := context.TODO()
		addr, wait, err := api.Put(ctx, content, exp.MimeType, toEncrypt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		err = wait(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		resp := testGet(t, api, addr.Hex(), "")
		checkResponse(t, resp, exp)
	})
}

//TestResolver实现Resolver接口，并返回给定的
//如果设置了哈希，则返回“找不到名称”错误
type testResolveValidator struct {
	hash *common.Hash
}

func newTestResolveValidator(addr string) *testResolveValidator {
	r := &testResolveValidator{}
	if addr != "" {
		hash := common.HexToHash(addr)
		r.hash = &hash
	}
	return r
}

func (t *testResolveValidator) Resolve(addr string) (common.Hash, error) {
	if t.hash == nil {
		return common.Hash{}, fmt.Errorf("DNS name not found: %q", addr)
	}
	return *t.hash, nil
}

func (t *testResolveValidator) Owner(node [32]byte) (addr common.Address, err error) {
	return
}
func (t *testResolveValidator) HeaderByNumber(context.Context, *big.Int) (header *types.Header, err error) {
	return
}

//测试优先权测试解析可以包含内容哈希的URI
//或姓
func TestAPIResolve(t *testing.T) {
	ensAddr := "swarm.eth"
	hashAddr := "1111111111111111111111111111111111111111111111111111111111111111"
	resolvedAddr := "2222222222222222222222222222222222222222222222222222222222222222"
	doesResolve := newTestResolveValidator(resolvedAddr)
	doesntResolve := newTestResolveValidator("")

	type test struct {
		desc      string
		dns       Resolver
		addr      string
		immutable bool
		result    string
		expectErr error
	}

	tests := []*test{
		{
			desc:   "DNS not configured, hash address, returns hash address",
			dns:    nil,
			addr:   hashAddr,
			result: hashAddr,
		},
		{
			desc:      "DNS not configured, ENS address, returns error",
			dns:       nil,
			addr:      ensAddr,
			expectErr: errors.New(`no DNS to resolve name: "swarm.eth"`),
		},
		{
			desc:   "DNS configured, hash address, hash resolves, returns resolved address",
			dns:    doesResolve,
			addr:   hashAddr,
			result: resolvedAddr,
		},
		{
			desc:      "DNS configured, immutable hash address, hash resolves, returns hash address",
			dns:       doesResolve,
			addr:      hashAddr,
			immutable: true,
			result:    hashAddr,
		},
		{
			desc:   "DNS configured, hash address, hash doesn't resolve, returns hash address",
			dns:    doesntResolve,
			addr:   hashAddr,
			result: hashAddr,
		},
		{
			desc:   "DNS configured, ENS address, name resolves, returns resolved address",
			dns:    doesResolve,
			addr:   ensAddr,
			result: resolvedAddr,
		},
		{
			desc:      "DNS configured, immutable ENS address, name resolves, returns error",
			dns:       doesResolve,
			addr:      ensAddr,
			immutable: true,
			expectErr: errors.New(`immutable address not a content hash: "swarm.eth"`),
		},
		{
			desc:      "DNS configured, ENS address, name doesn't resolve, returns error",
			dns:       doesntResolve,
			addr:      ensAddr,
			expectErr: errors.New(`DNS name not found: "swarm.eth"`),
		},
	}
	for _, x := range tests {
		t.Run(x.desc, func(t *testing.T) {
			api := &API{dns: x.dns}
			uri := &URI{Addr: x.addr, Scheme: "bzz"}
			if x.immutable {
				uri.Scheme = "bzz-immutable"
			}
			res, err := api.ResolveURI(context.TODO(), uri, "")
			if err == nil {
				if x.expectErr != nil {
					t.Fatalf("expected error %q, got result %q", x.expectErr, res)
				}
				if res.String() != x.result {
					t.Fatalf("expected result %q, got %q", x.result, res)
				}
			} else {
				if x.expectErr == nil {
					t.Fatalf("expected no error, got %q", err)
				}
				if err.Error() != x.expectErr.Error() {
					t.Fatalf("expected error %q, got %q", x.expectErr, err)
				}
			}
		})
	}
}

func TestMultiResolver(t *testing.T) {
	doesntResolve := newTestResolveValidator("")

	ethAddr := "swarm.eth"
	ethHash := "0x2222222222222222222222222222222222222222222222222222222222222222"
	ethResolve := newTestResolveValidator(ethHash)

	testAddr := "swarm.test"
	testHash := "0x1111111111111111111111111111111111111111111111111111111111111111"
	testResolve := newTestResolveValidator(testHash)

	tests := []struct {
		desc   string
		r      Resolver
		addr   string
		result string
		err    error
	}{
		{
			desc: "No resolvers, returns error",
			r:    NewMultiResolver(),
			err:  NewNoResolverError(""),
		},
		{
			desc:   "One default resolver, returns resolved address",
			r:      NewMultiResolver(MultiResolverOptionWithResolver(ethResolve, "")),
			addr:   ethAddr,
			result: ethHash,
		},
		{
			desc: "Two default resolvers, returns resolved address",
			r: NewMultiResolver(
				MultiResolverOptionWithResolver(ethResolve, ""),
				MultiResolverOptionWithResolver(ethResolve, ""),
			),
			addr:   ethAddr,
			result: ethHash,
		},
		{
			desc: "Two default resolvers, first doesn't resolve, returns resolved address",
			r: NewMultiResolver(
				MultiResolverOptionWithResolver(doesntResolve, ""),
				MultiResolverOptionWithResolver(ethResolve, ""),
			),
			addr:   ethAddr,
			result: ethHash,
		},
		{
			desc: "Default resolver doesn't resolve, tld resolver resolve, returns resolved address",
			r: NewMultiResolver(
				MultiResolverOptionWithResolver(doesntResolve, ""),
				MultiResolverOptionWithResolver(ethResolve, "eth"),
			),
			addr:   ethAddr,
			result: ethHash,
		},
		{
			desc: "Three TLD resolvers, third resolves, returns resolved address",
			r: NewMultiResolver(
				MultiResolverOptionWithResolver(doesntResolve, "eth"),
				MultiResolverOptionWithResolver(doesntResolve, "eth"),
				MultiResolverOptionWithResolver(ethResolve, "eth"),
			),
			addr:   ethAddr,
			result: ethHash,
		},
		{
			desc: "One TLD resolver doesn't resolve, returns error",
			r: NewMultiResolver(
				MultiResolverOptionWithResolver(doesntResolve, ""),
				MultiResolverOptionWithResolver(ethResolve, "eth"),
			),
			addr:   ethAddr,
			result: ethHash,
		},
		{
			desc: "One defautl and one TLD resolver, all doesn't resolve, returns error",
			r: NewMultiResolver(
				MultiResolverOptionWithResolver(doesntResolve, ""),
				MultiResolverOptionWithResolver(doesntResolve, "eth"),
			),
			addr:   ethAddr,
			result: ethHash,
			err:    errors.New(`DNS name not found: "swarm.eth"`),
		},
		{
			desc: "Two TLD resolvers, both resolve, returns resolved address",
			r: NewMultiResolver(
				MultiResolverOptionWithResolver(ethResolve, "eth"),
				MultiResolverOptionWithResolver(testResolve, "test"),
			),
			addr:   testAddr,
			result: testHash,
		},
		{
			desc: "One TLD resolver, no default resolver, returns error for different TLD",
			r: NewMultiResolver(
				MultiResolverOptionWithResolver(ethResolve, "eth"),
			),
			addr: testAddr,
			err:  NewNoResolverError("test"),
		},
	}
	for _, x := range tests {
		t.Run(x.desc, func(t *testing.T) {
			res, err := x.r.Resolve(x.addr)
			if err == nil {
				if x.err != nil {
					t.Fatalf("expected error %q, got result %q", x.err, res.Hex())
				}
				if res.Hex() != x.result {
					t.Fatalf("expected result %q, got %q", x.result, res.Hex())
				}
			} else {
				if x.err == nil {
					t.Fatalf("expected no error, got %q", err)
				}
				if err.Error() != x.err.Error() {
					t.Fatalf("expected error %q, got %q", x.err, err)
				}
			}
		})
	}
}

func TestDecryptOriginForbidden(t *testing.T) {
	ctx := context.TODO()
	ctx = sctx.SetHost(ctx, "swarm-gateways.net")

	me := &ManifestEntry{
		Access: &AccessEntry{Type: AccessTypePass},
	}

	api := NewAPI(nil, nil, nil, nil)

	f := api.Decryptor(ctx, "")
	err := f(me)
	if err != ErrDecryptDomainForbidden {
		t.Fatalf("should fail with ErrDecryptDomainForbidden, got %v", err)
	}
}

func TestDecryptOrigin(t *testing.T) {
	for _, v := range []struct {
		host        string
		expectError error
	}{
		{
			host:        "localhost",
			expectError: ErrDecrypt,
		},
		{
			host:        "127.0.0.1",
			expectError: ErrDecrypt,
		},
		{
			host:        "swarm-gateways.net",
			expectError: ErrDecryptDomainForbidden,
		},
	} {
		ctx := context.TODO()
		ctx = sctx.SetHost(ctx, v.host)

		me := &ManifestEntry{
			Access: &AccessEntry{Type: AccessTypePass},
		}

		api := NewAPI(nil, nil, nil, nil)

		f := api.Decryptor(ctx, "")
		err := f(me)
		if err != v.expectError {
			t.Fatalf("should fail with %v, got %v", v.expectError, err)
		}
	}
}
