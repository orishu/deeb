// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: controller.proto

package controller

import (
	context "context"
	fmt "fmt"
	_ "github.com/gogo/googleapis/google/api"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/gogo/protobuf/proto"
	types "github.com/gogo/protobuf/types"
	golang_proto "github.com/golang/protobuf/proto"
	_ "github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger/options"
	_ "github.com/mwitkow/go-proto-validators"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	io "io"
	math "math"
	math_bits "math/bits"
	raftpb "raftpb"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = golang_proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type StatusCode int32

const (
	StatusCode_OK    StatusCode = 0
	StatusCode_ERROR StatusCode = 1
)

var StatusCode_name = map[int32]string{
	0: "OK",
	1: "ERROR",
}

var StatusCode_value = map[string]int32{
	"OK":    0,
	"ERROR": 1,
}

func (x StatusCode) String() string {
	return proto.EnumName(StatusCode_name, int32(x))
}

func (StatusCode) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_ed7f10298fa1d90f, []int{0}
}

type StatusResponse struct {
	Code                 StatusCode `protobuf:"varint,1,opt,name=code,proto3,enum=api.StatusCode" json:"code,omitempty"`
	XXX_NoUnkeyedLiteral struct{}   `json:"-"`
	XXX_unrecognized     []byte     `json:"-"`
	XXX_sizecache        int32      `json:"-"`
}

func (m *StatusResponse) Reset()         { *m = StatusResponse{} }
func (m *StatusResponse) String() string { return proto.CompactTextString(m) }
func (*StatusResponse) ProtoMessage()    {}
func (*StatusResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_ed7f10298fa1d90f, []int{0}
}
func (m *StatusResponse) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *StatusResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_StatusResponse.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *StatusResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_StatusResponse.Merge(m, src)
}
func (m *StatusResponse) XXX_Size() int {
	return m.Size()
}
func (m *StatusResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_StatusResponse.DiscardUnknown(m)
}

var xxx_messageInfo_StatusResponse proto.InternalMessageInfo

func (m *StatusResponse) GetCode() StatusCode {
	if m != nil {
		return m.Code
	}
	return StatusCode_OK
}

func (*StatusResponse) XXX_MessageName() string {
	return "api.StatusResponse"
}
func init() {
	proto.RegisterEnum("api.StatusCode", StatusCode_name, StatusCode_value)
	golang_proto.RegisterEnum("api.StatusCode", StatusCode_name, StatusCode_value)
	proto.RegisterType((*StatusResponse)(nil), "api.StatusResponse")
	golang_proto.RegisterType((*StatusResponse)(nil), "api.StatusResponse")
}

func init() { proto.RegisterFile("controller.proto", fileDescriptor_ed7f10298fa1d90f) }
func init() { golang_proto.RegisterFile("controller.proto", fileDescriptor_ed7f10298fa1d90f) }

var fileDescriptor_ed7f10298fa1d90f = []byte{
	// 419 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x74, 0x92, 0x41, 0x6b, 0x14, 0x31,
	0x14, 0xc7, 0xcd, 0xb6, 0x5d, 0x31, 0x87, 0xed, 0x1a, 0xa5, 0xc8, 0x2a, 0xd3, 0x52, 0x11, 0xa4,
	0xb8, 0x89, 0x1d, 0xd1, 0x83, 0x9e, 0xb4, 0x14, 0x04, 0x29, 0x85, 0xe9, 0xcd, 0x4b, 0xc9, 0xcc,
	0xbc, 0x4d, 0x43, 0x67, 0xe6, 0x85, 0x24, 0xb3, 0x8b, 0x57, 0x3f, 0x82, 0x7e, 0x21, 0x8f, 0x3d,
	0x0a, 0x7e, 0x01, 0xd9, 0xfa, 0x41, 0x64, 0x32, 0xa9, 0x2d, 0x8b, 0x3d, 0xe5, 0xf1, 0xff, 0xff,
	0xde, 0xe4, 0xcd, 0xff, 0x85, 0x8e, 0x0b, 0x6c, 0xbc, 0xc5, 0xaa, 0x02, 0xcb, 0x8d, 0x45, 0x8f,
	0x6c, 0x4d, 0x1a, 0x3d, 0x79, 0xac, 0x10, 0x55, 0x05, 0x22, 0x48, 0x79, 0x3b, 0x13, 0x50, 0x1b,
	0xff, 0xa5, 0x27, 0x26, 0xdb, 0xab, 0xa6, 0xd7, 0x35, 0x38, 0x2f, 0x6b, 0x13, 0x81, 0x64, 0x15,
	0x28, 0x5b, 0x2b, 0xbd, 0xc6, 0x26, 0xfa, 0x3b, 0xab, 0xfe, 0x4c, 0x43, 0x55, 0x9e, 0xd6, 0xd2,
	0x9d, 0x47, 0xe2, 0x49, 0x24, 0xa4, 0xd1, 0x42, 0x36, 0x0d, 0xfa, 0xd0, 0xee, 0xa2, 0xfb, 0x22,
	0x1c, 0xc5, 0x54, 0x41, 0x33, 0x75, 0x0b, 0xa9, 0x14, 0x58, 0x81, 0x26, 0x10, 0xff, 0xa1, 0x1f,
	0x2a, 0x54, 0x18, 0x4a, 0xd1, 0x55, 0x51, 0x7d, 0xa3, 0xb4, 0x3f, 0x6b, 0x73, 0x5e, 0x60, 0x2d,
	0xea, 0x85, 0xf6, 0xe7, 0xb8, 0x10, 0x0a, 0xa7, 0xc1, 0x9c, 0xce, 0x65, 0xa5, 0x4b, 0xe9, 0xd1,
	0x3a, 0xf1, 0xaf, 0x8c, 0x7d, 0xf7, 0xad, 0x9c, 0x79, 0x93, 0x8b, 0xee, 0xe8, 0xa5, 0xdd, 0xd7,
	0x74, 0x74, 0xe2, 0xa5, 0x6f, 0x5d, 0x06, 0xce, 0x60, 0xe3, 0x80, 0x3d, 0xa5, 0xeb, 0x05, 0x96,
	0xf0, 0x88, 0xec, 0x90, 0xe7, 0xa3, 0x74, 0x93, 0x4b, 0xa3, 0x79, 0x8f, 0x1c, 0x60, 0x09, 0x59,
	0x30, 0xf7, 0xb6, 0x29, 0xbd, 0xd6, 0xd8, 0x90, 0x0e, 0x8e, 0x3f, 0x8d, 0xef, 0xb0, 0x7b, 0x74,
	0xe3, 0x30, 0xcb, 0x8e, 0xb3, 0x31, 0x49, 0x4f, 0xe9, 0xe8, 0xa0, 0xdf, 0xce, 0x09, 0xd8, 0xb9,
	0x2e, 0x80, 0x1d, 0xd1, 0x61, 0xdf, 0xc2, 0xb6, 0x78, 0x9f, 0x10, 0xbf, 0xca, 0x90, 0x1f, 0x76,
	0x1b, 0x9a, 0x3c, 0xb8, 0x71, 0xd7, 0xd5, 0x38, 0xbb, 0x5b, 0x5f, 0x7f, 0xfd, 0xf9, 0x3e, 0x18,
	0xb3, 0x51, 0xc8, 0x73, 0xbe, 0x2f, 0x5c, 0xf0, 0xd3, 0xb7, 0x74, 0x3d, 0x93, 0x33, 0xcf, 0x52,
	0x7a, 0xf7, 0x08, 0x9c, 0x93, 0x0a, 0xd8, 0x26, 0xef, 0xff, 0x8f, 0x47, 0x61, 0x72, 0xcb, 0x45,
	0x1f, 0x3e, 0x7e, 0x7b, 0x4f, 0xd9, 0x46, 0xba, 0xb6, 0xcf, 0x5f, 0xee, 0x91, 0xc1, 0xc5, 0x32,
	0x21, 0x3f, 0x97, 0x09, 0xf9, 0xbd, 0x4c, 0xc8, 0x8f, 0xcb, 0x84, 0x5c, 0x5c, 0x26, 0xe4, 0xf3,
	0xb3, 0x1b, 0x11, 0xa3, 0xd5, 0xee, 0xac, 0x15, 0x25, 0x40, 0xde, 0x0d, 0xf1, 0xee, 0xfa, 0xd9,
	0xe5, 0xc3, 0xf0, 0xe5, 0x57, 0x7f, 0x03, 0x00, 0x00, 0xff, 0xff, 0x14, 0x4d, 0xf7, 0x1f, 0x8b,
	0x02, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// ControlServiceClient is the client API for ControlService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type ControlServiceClient interface {
	Status(ctx context.Context, in *types.Empty, opts ...grpc.CallOption) (*StatusResponse, error)
}

type controlServiceClient struct {
	cc *grpc.ClientConn
}

func NewControlServiceClient(cc *grpc.ClientConn) ControlServiceClient {
	return &controlServiceClient{cc}
}

func (c *controlServiceClient) Status(ctx context.Context, in *types.Empty, opts ...grpc.CallOption) (*StatusResponse, error) {
	out := new(StatusResponse)
	err := c.cc.Invoke(ctx, "/api.ControlService/Status", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ControlServiceServer is the server API for ControlService service.
type ControlServiceServer interface {
	Status(context.Context, *types.Empty) (*StatusResponse, error)
}

// UnimplementedControlServiceServer can be embedded to have forward compatible implementations.
type UnimplementedControlServiceServer struct {
}

func (*UnimplementedControlServiceServer) Status(ctx context.Context, req *types.Empty) (*StatusResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Status not implemented")
}

func RegisterControlServiceServer(s *grpc.Server, srv ControlServiceServer) {
	s.RegisterService(&_ControlService_serviceDesc, srv)
}

func _ControlService_Status_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(types.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ControlServiceServer).Status(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.ControlService/Status",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ControlServiceServer).Status(ctx, req.(*types.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

var _ControlService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "api.ControlService",
	HandlerType: (*ControlServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Status",
			Handler:    _ControlService_Status_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "controller.proto",
}

// RaftClient is the client API for Raft service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type RaftClient interface {
	Message(ctx context.Context, in *raftpb.Message, opts ...grpc.CallOption) (*types.Empty, error)
}

type raftClient struct {
	cc *grpc.ClientConn
}

func NewRaftClient(cc *grpc.ClientConn) RaftClient {
	return &raftClient{cc}
}

func (c *raftClient) Message(ctx context.Context, in *raftpb.Message, opts ...grpc.CallOption) (*types.Empty, error) {
	out := new(types.Empty)
	err := c.cc.Invoke(ctx, "/api.Raft/Message", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RaftServer is the server API for Raft service.
type RaftServer interface {
	Message(context.Context, *raftpb.Message) (*types.Empty, error)
}

// UnimplementedRaftServer can be embedded to have forward compatible implementations.
type UnimplementedRaftServer struct {
}

func (*UnimplementedRaftServer) Message(ctx context.Context, req *raftpb.Message) (*types.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Message not implemented")
}

func RegisterRaftServer(s *grpc.Server, srv RaftServer) {
	s.RegisterService(&_Raft_serviceDesc, srv)
}

func _Raft_Message_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(raftpb.Message)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RaftServer).Message(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/api.Raft/Message",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RaftServer).Message(ctx, req.(*raftpb.Message))
	}
	return interceptor(ctx, in, info, handler)
}

var _Raft_serviceDesc = grpc.ServiceDesc{
	ServiceName: "api.Raft",
	HandlerType: (*RaftServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Message",
			Handler:    _Raft_Message_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "controller.proto",
}

func (m *StatusResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *StatusResponse) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *StatusResponse) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.XXX_unrecognized != nil {
		i -= len(m.XXX_unrecognized)
		copy(dAtA[i:], m.XXX_unrecognized)
	}
	if m.Code != 0 {
		i = encodeVarintController(dAtA, i, uint64(m.Code))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func encodeVarintController(dAtA []byte, offset int, v uint64) int {
	offset -= sovController(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *StatusResponse) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Code != 0 {
		n += 1 + sovController(uint64(m.Code))
	}
	if m.XXX_unrecognized != nil {
		n += len(m.XXX_unrecognized)
	}
	return n
}

func sovController(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozController(x uint64) (n int) {
	return sovController(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *StatusResponse) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowController
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: StatusResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: StatusResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Code", wireType)
			}
			m.Code = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowController
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Code |= StatusCode(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipController(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthController
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthController
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			m.XXX_unrecognized = append(m.XXX_unrecognized, dAtA[iNdEx:iNdEx+skippy]...)
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipController(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowController
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowController
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowController
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthController
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupController
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthController
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthController        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowController          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupController = fmt.Errorf("proto: unexpected end of group")
)
