package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const (
	SG_GET_VERSION_NUM   = 0x2282
	SG_IO                = 0x2285
	SG_INFO_OK_MASK      = 0x1
	SG_INFO_OK           = 0x0
	SG_DXFER_FROM_DEV    = -3
	SG_DXFER_TO_FROM_DEV = -4
	INQ_CMD_CODE         = 0x12
	INQ_REPLY_LEN        = 96
	SENSE_BUF_LEN        = 32
	TIMEOUT_20_SECS      = 20000
)

var Debug = false

// pahole for sg_io_hdr_t on amd64
/*
 * struct sg_io_hdr {
 * 	int                        interface_id;         //     0     4
 * 	int                        dxfer_direction;      //     4     4
 * 	unsigned char              cmd_len;              //     8     1
 * 	unsigned char              mx_sb_len;            //     9     1
 * 	short unsigned int         iovec_count;          //    10     2
 * 	unsigned int               dxfer_len;            //    12     4
 * 	void *                     dxferp;               //    16     8
 * 	unsigned char *            cmdp;                 //    24     8
 * 	unsigned char *            sbp;                  //    32     8
 * 	unsigned int               timeout;              //    40     4
 * 	unsigned int               flags;                //    44     4
 * 	int                        pack_id;              //    48     4
 *
 * 	// XXX 4 bytes hole, try to pack
 *
 * 	void *                     usr_ptr;              //    56     8
 * 	// --- cacheline 1 boundary (64 bytes) ---
 * 	unsigned char              status;               //    64     1
 * 	unsigned char              masked_status;        //    65     1
 * 	unsigned char              msg_status;           //    66     1
 * 	unsigned char              sb_len_wr;            //    67     1
 * 	short unsigned int         host_status;          //    68     2
 * 	short unsigned int         driver_status;        //    70     2
 * 	int                        resid;                //    72     4
 * 	unsigned int               duration;             //    76     4
 * 	unsigned int               info;                 //    80     4
 *
 * 	// size: 88, cachelines: 2, members: 22
 * 	// sum members: 80, holes: 1, sum holes: 4
 * 	// padding: 4
 * 	// last cacheline: 24 bytes
 * };
 */

// SgIoHdr is our version of sg_io_hdr_t that gets passed to the SG_IO ioctl
type SgIoHdr struct {
	InterfaceID    int32
	DxferDirection int32
	CmdLen         uint8
	MxSbLen        uint8
	IovecCount     uint16
	DxferLen       uint32
	Dxferp         *byte
	Cmdp           *uint8
	Sbp            *byte
	Timeout        uint32
	Flags          uint32
	PackID         int32
	Pad0           [4]byte
	UsrPtr         *byte
	Status         uint8
	MaskedStatus   uint8
	MsgStatus      uint8
	SbLenWr        uint8
	HostStatus     uint16
	DriverStatus   uint16
	Resid          int32
	Duration       uint32
	Info           uint32
}

func Inquire(device string) error {
	inqBuf := make([]byte, INQ_REPLY_LEN)
	senseBuf := make([]byte, SENSE_BUF_LEN)
	inqCmdBlk := []uint8{INQ_CMD_CODE, 0, 0, 0, INQ_REPLY_LEN, 0}
	ioHdr := SgIoHdr{
		InterfaceID:    int32('S'),
		CmdLen:         uint8(len(inqCmdBlk)),
		MxSbLen:        SENSE_BUF_LEN,
		DxferDirection: SG_DXFER_FROM_DEV,
		DxferLen:       INQ_REPLY_LEN,
		Dxferp:         &inqBuf[0],
		Cmdp:           &inqCmdBlk[0],
		Sbp:            &senseBuf[0],
		Timeout:        TIMEOUT_20_SECS,
	}

	f, err := openScsiDevice(device)
	if err != nil {
		return err
	}
	defer f.Close()

	err = ioctl(f.Fd(), SG_IO, uintptr(unsafe.Pointer(&ioHdr)))
	if err != nil {
		return err
	}

	err = checkOK(ioHdr, &senseBuf)
	if err != nil {
		return err
	}

	if Debug {
		fmt.Printf("INQUIRY response:\n%v", hex.Dump(inqBuf))
	}
	fmt.Println("Vendor   ID:", parseString(inqBuf[8:][:8]))
	fmt.Println("Product  ID:", parseString(inqBuf[16:][:16]))
	fmt.Println("Product rev:", parseString(inqBuf[32:][:4]))

	inqCmdBlk = []uint8{INQ_CMD_CODE, 0x01, 0x80, 0, INQ_REPLY_LEN, 0}
	ioHdr.Cmdp = &inqCmdBlk[0]
	err = ioctl(f.Fd(), SG_IO, uintptr(unsafe.Pointer(&ioHdr)))
	if err != nil {
		return err
	}

	err = checkOK(ioHdr, &senseBuf)
	if err != nil {
		return err
	}

	if Debug {
		fmt.Printf("INQUIRY response:\n%v", hex.Dump(inqBuf))
	}
	fmt.Println("Serial:", parseString(inqBuf[4:][:12]))

	return nil
}

func TestUnitReady(device string) error {
	senseBuf := make([]byte, SENSE_BUF_LEN)
	inqCmdBlk := []uint8{0, 0, 0, 0, 0, 0}
	ioHdr := SgIoHdr{
		InterfaceID:    int32('S'),
		CmdLen:         uint8(len(inqCmdBlk)),
		MxSbLen:        SENSE_BUF_LEN,
		DxferDirection: SG_DXFER_FROM_DEV,
		Cmdp:           &inqCmdBlk[0],
		Sbp:            &senseBuf[0],
		Timeout:        TIMEOUT_20_SECS,
	}

	f, err := openScsiDevice(device)
	if err != nil {
		return err
	}
	defer f.Close()

	err = ioctl(f.Fd(), SG_IO, uintptr(unsafe.Pointer(&ioHdr)))
	if err != nil {
		return err
	}

	err = checkOK(ioHdr, &senseBuf)
	if err != nil {
		return err
	}

	fmt.Println("TUR: OK")
	return nil
}

func checkOK(i SgIoHdr, s *[]byte) error {
	if (i.Info & SG_INFO_OK_MASK) != SG_INFO_OK {
		if i.SbLenWr > 0 {
			fmt.Printf("SENSE:\n%v\n", dumpHex(*s))
			fmt.Println(GetErrString((*s)[12], (*s)[13]))
		}
		if i.MaskedStatus != 0 {
			fmt.Println("SCSI status:", i.Status)
		}
		if i.HostStatus != 0 {
			fmt.Println("host status:", i.HostStatus)
		}
		if i.DriverStatus != 0 {
			fmt.Println("driver status:", i.DriverStatus)
		}
		return errors.New("SCSI response not ok")
	}
	if Debug {
		fmt.Println(GetErrString((*s)[12], (*s)[13]))
	}
	return nil
}

func ioctl(fd, cmd, ptr uintptr) error {
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, cmd, ptr)
	if err != 0 {
		return err
	}
	return nil
}

func openScsiDevice(fname string) (*os.File, error) {
	f, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	var version uint32
	if (ioctl(f.Fd(), SG_GET_VERSION_NUM, uintptr(unsafe.Pointer(&version))) != nil) || (version < 30000) {
		return nil, errors.New("device does not appear to be an sg device")
	}
	return f, nil
}
