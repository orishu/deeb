package server

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	pb "github.com/orishu/deeb/api"
	nd "github.com/orishu/deeb/internal/node"
)

type controlService struct {
	pb.UnimplementedControlServiceServer
	node *nd.Node
}

func (c controlService) Status(context.Context, *emptypb.Empty) (*pb.StatusResponse, error) {
	resp := pb.StatusResponse{Code: pb.StatusCode_OK}
	return &resp, nil
}

func (c controlService) AddPeer(ctx context.Context, req *pb.AddPeerRequest) (*emptypb.Empty, error) {
	err := c.node.AddNode(ctx, req.Id, nd.NodeInfo{Addr: req.Addr, Port: req.Port})
	return &emptypb.Empty{}, err
}

func (c controlService) ExecuteSQL(ctx context.Context, req *pb.ExecuteSQLRequest) (*emptypb.Empty, error) {
	err := c.node.WriteQuery(ctx, req.Sql)
	return &emptypb.Empty{}, err
}

func (c controlService) QuerySQL(req *pb.QuerySQLRequest, srv pb.ControlService_QuerySQLServer) error {
	rows, err := c.node.ReadQuery(srv.Context(), req.Sql)
	if err != nil {
		return err
	}
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return err
	}
	for rows.Next() {
		values := make([]interface{}, 0, len(columnTypes))
		for _, ct := range columnTypes {
			scanType := ct.ScanType()
			if scanType == reflect.TypeOf(nil) {
				switch ct.DatabaseTypeName() {
				case "INTEGER", "INT":
					scanType = reflect.TypeOf(int(1))
				case "REAL":
					scanType = reflect.TypeOf(float64(0))
				case "TEXT":
					scanType = reflect.TypeOf("")
				case "BLOB":
					scanType = reflect.TypeOf([]byte{})
				}
			}
			values = append(values, reflect.New(scanType).Interface())
		}
		err := rows.Scan(values...)
		if err != nil {
			return err
		}
		cells := make([]*pb.Row_Cell, 0, len(values))
		for _, value := range values {
			switch v := value.(type) {
			case *string:
				cells = append(cells, &pb.Row_Cell{Value: &pb.Row_Cell_Str{Str: *v}})
			case *[]byte:
				cells = append(cells, &pb.Row_Cell{Value: &pb.Row_Cell_By{By: *v}})
			case *int:
				cells = append(cells, &pb.Row_Cell{Value: &pb.Row_Cell_I64{I64: int64(*v)}})
			case *uint:
				cells = append(cells, &pb.Row_Cell{Value: &pb.Row_Cell_I64{I64: int64(*v)}})
			case *uint64:
				cells = append(cells, &pb.Row_Cell{Value: &pb.Row_Cell_I64{I64: int64(*v)}})
			case *int64:
				cells = append(cells, &pb.Row_Cell{Value: &pb.Row_Cell_I64{I64: *v}})
			case *int8:
				cells = append(cells, &pb.Row_Cell{Value: &pb.Row_Cell_I32{I32: int32(*v)}})
			case *int16:
				cells = append(cells, &pb.Row_Cell{Value: &pb.Row_Cell_I32{I32: int32(*v)}})
			case *int32:
				cells = append(cells, &pb.Row_Cell{Value: &pb.Row_Cell_I32{I32: *v}})
			case *uint8:
				cells = append(cells, &pb.Row_Cell{Value: &pb.Row_Cell_I32{I32: int32(*v)}})
			case *uint16:
				cells = append(cells, &pb.Row_Cell{Value: &pb.Row_Cell_I32{I32: int32(*v)}})
			case *uint32:
				cells = append(cells, &pb.Row_Cell{Value: &pb.Row_Cell_I32{I32: int32(*v)}})
			case *bool:
				cells = append(cells, &pb.Row_Cell{Value: &pb.Row_Cell_B{B: *v}})
			case *time.Time:
				cells = append(cells, &pb.Row_Cell{
					Value: &pb.Row_Cell_Ts{
						Ts: &timestamppb.Timestamp{
							Seconds: v.Unix(),
							Nanos:   int32(v.UnixNano()),
						},
					},
				})
			default:
				return fmt.Errorf("unsupported scan value type: %T", v)
			}
		}
		resp := pb.QuerySQLResponse{Row: &pb.Row{Cells: cells}}
		if err := srv.Send(&resp); err != nil {
			return err
		}
	}
	return rows.Err()
}
