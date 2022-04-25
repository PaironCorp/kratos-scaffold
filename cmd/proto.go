package cmd

import (
	"strings"

	"github.com/YoogoC/kratos-scaffold/generator"
	"github.com/YoogoC/kratos-scaffold/pkg/field"

	"github.com/spf13/cobra"
)

var (
	outProtoPath string
)

func newProtoCmd() *cobra.Command {
	var protoCmd = &cobra.Command{
		Use:   "proto [NAME]",
		Short: "proto generate req and res model, crud service and xx.pb.go,xx_grpc.pb.go,[xx_http.pb.go].",
		Long: `proto generate req and res model, crud service and xx.pb.go,xx_grpc.pb.go,[xx_http.pb.go].
kratos-scaffold  user -o api/user/v1/user.proto id:int64:eq,in name:string:contains age:int32:gte,lte -g -h`,
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		Run: func(cmd *cobra.Command, args []string) {
			modelName := args[0]
			proto := generator.NewProto(modelName, outProtoPath, parseFields(args[1:]))
			err := proto.Generate()
			if err != nil {
				panic(err)
			}
		},
	}

	addProtoFlags(protoCmd)

	return protoCmd
}

func addProtoFlags(protoCmd *cobra.Command) {
	protoCmd.PersistentFlags().StringVarP(&outProtoPath, "out", "o", "", "proto out path")
}

func parseFields(strs []string) []field.Field {
	var fs []field.Field
	for _, str := range strs {
		ss := strings.Split(str, ":")
		var pres []field.Predicate
		if len(ss) > 2 {
			for _, p := range strings.Split(ss[2], ",") {
				pres = append(pres, field.Predicate{
					Name:      ss[0],
					Type:      field.NewPredicateType(p),
					FieldType: ss[1],
				})
			}
		}
		fs = append(fs, field.Field{
			Name:       ss[0],
			FieldType:  ss[1],
			Predicates: pres,
		})
	}
	return fs
}
