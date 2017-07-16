package client

/*type DKG struct {*/
//private      *Private
//participants []*Identity
//n            int
//router       Router
//dkg          *dkg.DistKeyGenerator
//t            int
//}

//func NewDKG(priv *Private, index int, participants []*Identity, r Router) *DKG {
//d := &DKG{
//private:      priv,
//participants: participants,
//router:       r,
//n:            len(participants),
//t:            len(participants) / 2,
//}
//publics := make([]kyber.Point, len(participants))
//broadcastList := make([]*Identity, len(participants)-1)
//for i, p := range participants {
//publics[i] = p.Point()
//}
//d.router.RegisterProcessor(d)
//return d
//}

//func (d *DKG) Run() error {
//d.sendDeals()
//return nil
//}

//func (d *DKG) Process(id *Identity, cm *ClientMessage) {
//correct := DkgProtocol == PROTOCOL_RDKG &&
//cm.Rdkg != nil

//if !correct {
//return
//}
//dkgPacket := cm.Rdkg
//switch {
//case dkgPacket.Deal != nil:
//d.processDeal(id, dkgPacket.Deal)
//case dkgPacket.Response != nil:
//d.processResponse(id, dkgPacket.Response)
//case dkgPacket.Justification != nil:
//d.processJustification(id, dkgPacket.Justification)
//}

//}

//func (d *DKG) processDeal(id *Identity, deal *dkg.Deal) {
//resp, err := d.dkg.ProcessDeal(deal)
//if err != nil {
//slog.Print("dkg: deal error: " + err.Error())
//}
//msg := &ClientMessage{
//DkgProtocol: PROTOCOL_RDKG,
//Rdkg: &Rdkg{
//Response: resp,
//},
//}

//if err := d.router.Send(id, msg); err != nil {
//slog.Info("dkg: send response error", err)
//return
//}
//}

//func (d *DKG) ProcessResponse(id *Identity, deal *dkg.Response) {
//j, err := d.dkg.ProcessResponse(deal)
//if err != nil {
//slog.Print("dkg: response invalid:", err)
//}

//if j != nil {
//msg := &ClientMessage{
//DkgProtocol: PROTOCOL_RDKG,
//Rdkg: &Rdkg{
//Justification: j,
//},
//}
//if err := d.router.Broadcast(msg, d.participants); err != nil {
//slog.Info("dkg: broadcasting justification:", err)
//}
//}

//if d.dkg.Certified() {
//d.sendSecretCommits()
//}
//}

//func (d *DKG) sendDeals() {
//deals, err := d.dkg.Deals()
//if err != nil {
//return err
//}

//// sends the deal, with an overall timeout termination
//var timeout = time.Minute * 5
//var timeoutCh = make(chan bool)
//var global = make(chan error)
//for i, deal := range deals {
//id := d.participants[i]
//cm := &ClientMessage{
//DkgProtocol: PROTOCOL_RDKG,
//Deal:        deal,
//}
//c = d.router.Send(id, cm)
//go func() { global <- <-c }()
//}

//// run the timeout
//go func() { time.Sleep(timeout); timeoutCh <- true }()

//// wait to be sure that at least t+1 nodes got the deal
//nbConfirmedChannel := 0
//for nbConfirmedChannel < t+1 {
//select {
//case e := <-global:
//nbConfirmedChannel += 1
//continue
//case <-timeoutCh:
//panic("dkg: not enough participants online after timeout")
//}
//}
//}

//func (d *DKG) sendSecretCommits() {
//sc, err := d.dkg.SecretCommits()
//if err != nil {
//panic(err)
//}
//cm := &ClientMessage{
//DkgProtocol: PROTOCOL_RDKG,
//Rdkg: &Rdkg{
//SecretCommits: sc,
//},
//}
//d.router.Broadcast(d.participants, sc)
/*}*/
