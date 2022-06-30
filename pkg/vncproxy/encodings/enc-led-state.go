package encodings

import (
	"io"
	"yunion.io/x/log"
	"yunion.io/x/onecloud/pkg/vncproxy/common"
)

type EncLedStatePseudo struct {
	LedState uint8
}

func (pe *EncLedStatePseudo) Type() int32 {
	return int32(common.EncLedStatePseudo)
}
func (pe *EncLedStatePseudo) WriteTo(w io.Writer) (n int, err error) {
	w.Write([]byte{pe.LedState})
	return 1, nil
}
func (pe *EncLedStatePseudo) Read(pf *common.PixelFormat, rect *common.Rectangle, r *common.RfbReadHelper) (common.IEncoding, error) {
	if rect.Width*rect.Height == 0 {
		return pe, nil
	}
	u8, err := r.ReadUint8()
	pe.LedState = u8
	if err != nil {
		log.Errorf("error while reading led state: ", err)
		return pe, err
	}
	return pe, nil
}
