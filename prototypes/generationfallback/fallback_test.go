package generationfallback

import (
    "context"
    "encoding/json"
    "errors"
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"
)

type fakeGen struct {
    out Output
    err error
    calls int
}
func (f *fakeGen) Generate(_ context.Context, _ Input) (Output,error) {
    f.calls++
    return f.out,f.err
}

func input() Input {
    return Input{
        Synthetic:true, CrisisGatePassed:true, MetaConsent:true,
        EnableFallback:true, FallbackConsent:true,
        FixtureID:"adult-seeded-id", FixtureVersion:"fixture-v1",
        BriefVersion:2,Language:"id",
        Evidence:[]Evidence{{ID:"e1",Quote:"Saya lelah, tetapi masih mengikuti kuliah."}},
    }
}
func output() Output {
    return Output{Claims:[]Claim{{ID:"c1",Text:"Saya merasa lelah.",EvidenceIDs:[]string{"e1"}}}}
}

func TestMetaIsPrimary(t *testing.T) {
    meta:=&fakeGen{out:output()}
    secondary:=&fakeGen{out:output()}
    r:=Router{Primary:meta,Secondary:secondary,PrimaryModel:"muse-spark-open"}
    got,err:=r.Generate(context.Background(),input())
    if err!=nil || got.Origin!="live_meta" || got.FallbackUsed || meta.calls!=1 || secondary.calls!=0{
        t.Fatalf("unexpected outcome %+v %v",got,err)
    }
}

func TestFallbackOnlyForExplicitConsentAndUnavailablePrimary(t *testing.T) {
    for _,tc:=range []struct{name string; enabled,consent bool; wantCalls int}{
        {"opted in",true,true,1},
        {"default off",false,true,0},
        {"no consent",true,false,0},
    } {
        t.Run(tc.name,func(t *testing.T){
            m:=&fakeGen{err:ErrUnavailable}
            f:=&fakeGen{out:output()}
            req:=input(); req.EnableFallback=tc.enabled; req.FallbackConsent=tc.consent
            got,err:=(Router{Primary:m,Secondary:f,PrimaryModel:"meta"}).Generate(context.Background(),req)
            if f.calls!=tc.wantCalls {t.Fatalf("unexpected backup calls: %d",f.calls)}
            if tc.wantCalls>0 {
                if err!=nil ||got.Provider!=DeepSeekProvider ||got.Origin!="live_deepseek_fallback" ||!got.FallbackUsed {
                    t.Fatalf("missing provenance: %+v %v",got,err)
                }
            } else if !errors.Is(err,ErrUnavailable) {t.Fatalf("must fail closed: %v",err)}
        })
    }
}

func TestNoFallbackOnNonRetryableOrUnsafePrimary(t *testing.T) {
    for _,tc:=range []struct{name string; err error; out Output}{
        {"rejected policy",ErrUnsafe,Output{}},
        {"bad input",ErrBadInput,Output{}},
        {"unsupported output",nil,Output{Claims:[]Claim{{ID:"c1",Text:"Diagnosis invented",EvidenceIDs:[]string{"forged"}}}}},
    }{
        t.Run(tc.name,func(t *testing.T){
            m:=&fakeGen{err:tc.err,out:tc.out}
            f:=&fakeGen{out:output()}
            _,err:=(Router{Primary:m,Secondary:f,PrimaryModel:"muse"}).Generate(context.Background(),input())
            if err==nil || f.calls!=0 {t.Fatalf("unsafe silent fallback: %v, calls=%d",err,f.calls)}
        })
    }
}

func TestCrisisOrMissingMetaConsentMeansNoCalls(t *testing.T) {
    for _,mutate:=range []func(*Input){
        func(i *Input){i.CrisisGatePassed=false},
        func(i *Input){i.MetaConsent=false},
        func(i *Input){i.Synthetic=false},
        func(i *Input){i.Evidence[0].ID=""},
    }{
        i:=input();mutate(&i)
        a:=&fakeGen{out:output()};b:=&fakeGen{out:output()}
        _,err:=(Router{Primary:a,Secondary:b,PrimaryModel:"meta"}).Generate(context.Background(),i)
        if !errors.Is(err,ErrBadInput)||a.calls!=0||b.calls!=0{t.Fatalf("unsafe call: %v %+v %+v",err,a,b)}
    }
}

func TestCanceledContextSkipsFallback(t *testing.T) {
    ctx,cancel:=context.WithCancel(context.Background());cancel()
    a:=&fakeGen{err:ErrUnavailable};b:=&fakeGen{out:output()}
    _,err:=(Router{Primary:a,Secondary:b,PrimaryModel:"meta"}).Generate(ctx,input())
    if !errors.Is(err,context.Canceled)||a.calls!=0||b.calls!=0{t.Fatalf("cancellation failure: %v",err)}
}

func newMock(t *testing.T, status int, payload string, check func(*http.Request)) *DeepSeekClient {
    t.Helper()
    server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
        if check!=nil{check(r)}
        w.WriteHeader(status);_,_=w.Write([]byte(payload))
    }))
    t.Cleanup(server.Close)
    c,_:=NewDeepSeekClient("test-key")
    c.endpoint=server.URL
    return c
}

func mockEnvelope(t *testing.T,out any,model string,finish string) string {
    t.Helper()
    content,err:=json.Marshal(out);if err!=nil{t.Fatal(err)}
    envelope,err:=json.Marshal(map[string]any{
        "model":model,
        "choices":[]map[string]any{{"finish_reason":finish,"message":map[string]string{"content":string(content)}}},
    })
    if err!=nil{t.Fatal(err)}
    return string(envelope)
}

func TestDeepSeekExactAliasAndSafeJSONRequest(t *testing.T) {
    body:=mockEnvelope(t,output(),DeepSeekVersion,"stop")
    c:=newMock(t,200,body,func(r *http.Request){
        if r.Header.Get("Authorization")!="Bearer test-key"||r.Method!="POST"{t.Error("auth/method")}
        var req map[string]any
        if err:=json.NewDecoder(r.Body).Decode(&req);err!=nil{t.Fatal(err)}
        if req["model"]!="deepseek-flash"||req["response_format"]==nil{t.Fatal("missing alias / JSON mode")}
        thinking,ok:=req["thinking"].(map[string]any)
        if !ok||thinking["type"]!="disabled" {t.Fatal("thinking should be disabled for short structured task")}
    })
    got,err:=c.Generate(context.Background(),input())
    if err!=nil||len(got.Claims)!=1{t.Fatalf("bad response %+v %v",got,err)}
}

func TestDeepSeekFailClosedResponses(t *testing.T) {
    cases:=[]struct{name string;status int; body string; want error}{
        {"429",429,"rate limited",ErrUnavailable},
        {"503",503,"please retry",ErrUnavailable},
        {"401",401,"bad api key",ErrUnsafe},
        {"truncated",200,mockEnvelope(t,output(),DeepSeekVersion,"length"),ErrUnsafe},
        {"forged evidence",200,mockEnvelope(t,Output{Claims:[]Claim{{ID:"c1",Text:"X",EvidenceIDs:[]string{"bad"}}}},DeepSeekVersion,"stop"),ErrUnsafe},
        {"wrong model",200,mockEnvelope(t,output(),"unexpected-model","stop"),ErrUnsafe},
        {"html",200,"<h1>oops</h1>",ErrUnsafe},
    }
    for _,tc:=range cases{
        t.Run(tc.name,func(t *testing.T){
            c:=newMock(t,tc.status,tc.body,nil)
            _,err:=c.Generate(context.Background(),input())
            if !errors.Is(err,tc.want){t.Fatalf("bad error: %v",err)}
            if strings.Contains(err.Error(),"api key"){t.Fatal("provider body leaked")}
        })
    }
}

func TestDeepSeekRequiresOwnFallbackConsent(t *testing.T) {
    calls:=0
    c:=newMock(t,200,"",func(r *http.Request){calls++})
    i:=input();i.FallbackConsent=false
    if _,err:=c.Generate(context.Background(),i);!errors.Is(err,ErrBadInput){t.Fatalf("no guard: %v",err)}
    if calls!=0{t.Fatal("consentless network call")}
}

func TestDeepSeekContextCancellation(t *testing.T) {
    ctx,cancel:=context.WithCancel(context.Background());cancel()
    c:=newMock(t,200,"",func(r *http.Request){t.Error("must not call")})
    if _,err:=c.Generate(ctx,input());!errors.Is(err,context.Canceled){t.Fatalf("expected cancellation: %v",err)}
}
