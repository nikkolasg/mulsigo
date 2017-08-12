package client

import (
	"errors"
	"sync"
	"time"

	"github.com/dedis/kyber/random"
	"github.com/nikkolasg/mulsigo/slog"
	kyber "gopkg.in/dedis/kyber.v1"
	"gopkg.in/dedis/kyber.v1/share/pedersen/dkg"
)

type DKG struct {
	private      *Private
	participants []*Identity
	n            int
	t            int
	i            int
	router       *Router
	dkg          *dkg.DistKeyGenerator
	shareCh      chan *dkg.DistKeyShare
	done         bool
	sync.Mutex
}

func NewDKG(priv *Private, participants []*Identity, r *Router) (*DKG, error) {
	d := &DKG{
		private:      priv,
		participants: participants,
		router:       r,
		n:            len(participants),
		// default threshold
		t:       len(participants)/2 + 1,
		shareCh: make(chan *dkg.DistKeyShare),
	}
	publics := make([]kyber.Point, len(participants))
	idx := -1
	pubID := priv.Public.ID()
	for i, p := range participants {
		if idx == -1 && p.ID() == pubID {
			idx = i
		}
		publics[i] = p.Point()
	}
	if idx == -1 {
		return nil, errors.New("dkg: public key not found in participants list")
	}
	d.i = idx
	d.router.RegisterProcessor(d)
	var err error
	d.dkg, err = dkg.NewDistKeyGenerator(Group, d.private.Scalar(), publics, random.Stream, d.t)
	return d, err
}

func (d *DKG) Run() (*dkg.DistKeyShare, error) {
	if err := d.sendDeals(); err != nil {
		return nil, err
	}
	share := <-d.shareCh
	return share, nil
}

func (d *DKG) Process(id *Identity, cm *ClientMessage) {
	d.Lock()
	defer d.Unlock()
	correct := cm.Type == PROTOCOL_PDKG &&
		cm.PDkg != nil

	if !correct {
		return
	}
	dkgPacket := cm.PDkg
	switch {
	case dkgPacket.Deal != nil:
		d.processDeal(id, dkgPacket.Deal)
	case dkgPacket.Response != nil:
		d.processResponse(id, dkgPacket.Response)
	case dkgPacket.Justification != nil:
		d.processJustification(id, dkgPacket.Justification)
	}

}

func (d *DKG) processDeal(id *Identity, deal *dkg.Deal) {
	resp, err := d.dkg.ProcessDeal(deal)
	if err != nil {
		slog.Fatal("dkg: deal error: " + err.Error())
	}
	msg := &ClientMessage{
		Type: PROTOCOL_PDKG,
		PDkg: &PDkg{
			Response: resp,
		},
	}

	if err := d.router.Broadcast(msg, d.participants...); err != nil {
		slog.Info("dkg: send response error", err)
		return
	}
}

func (d *DKG) processResponse(id *Identity, deal *dkg.Response) {
	j, err := d.dkg.ProcessResponse(deal)
	if err != nil {
		slog.Print("dkg: response invalid:", err)
	}

	if j != nil {
		msg := &ClientMessage{
			Type: PROTOCOL_PDKG,
			PDkg: &PDkg{
				Justification: j,
			},
		}
		if err := d.router.Broadcast(msg, d.participants...); err != nil {
			slog.Info("dkg: broadcasting justification:", err)
		}
	}

	if d.dkg.Certified() {
		dks, err := d.dkg.DistKeyShare()
		if err != nil {
			panic(err)
		}
		d.done = true
		d.shareCh <- dks
	}
}

func (d *DKG) processJustification(id *Identity, j *dkg.Justification) {

}

func (d *DKG) sendDeals() error {
	deals, err := d.dkg.Deals()
	if err != nil {
		return err
	}

	// sends the deal, with an overall timeout termination
	var timeout = time.Minute * 5
	var timeoutCh = make(chan bool)
	var global = make(chan error)
	for i, deal := range deals {
		id := d.participants[i]
		cm := &ClientMessage{
			Type: PROTOCOL_PDKG,
			PDkg: &PDkg{
				Deal: deal,
			},
		}
		go func(i *Identity, c *ClientMessage) { global <- d.router.Send(i, c) }(id, cm)
	}

	// run the timeout
	go func() { time.Sleep(timeout); timeoutCh <- true }()

	// wait to be sure that at least t+1 nodes got the deal
	nbConfirmedNodes := 0
	for nbConfirmedNodes < d.t {
		select {
		case e := <-global:
			if err != nil {
				slog.Debug("dkg: error sending deal", e)
			}
			nbConfirmedNodes += 1
			slog.Infof("dkg[%d]: got %d confirmed node for my deal", d.i, nbConfirmedNodes)
			continue
		case <-timeoutCh:
			return errors.New("dkg: not enough participants online after timeout")
		}
	}
	return nil
}
