/*
Author ：zaniu(zzaniu@126.com)
Time   ：2022/3/9 21:41
Desc   :

    ......................我佛慈悲......................

                           _oo0oo_
                          o8888888o
                          88" . "88
                          (| -_- |)
                          0\  =  /0
                        ___/`---'\___
                      .' \\|     |// '.
                     / \\|||  :  |||// \
                    / _||||| -卍-|||||- \
                   |   | \\\  -  /// |   |
                   | \_|  ''\---/''  |_/ |
                   \  .-\__  '-'  ___/-. /
                 ___'. .'  /--.--\  `. .'___
              ."" '<  `.___\_<|>_/___.' >' "".
             | | :  `- \`.;`\ _ /`;.`/ - ` : | |
             \  \ `_.   \_ __\ /__ _/   .-` /  /
         =====`-.____`.___ \_____/___.-`___.-'=====
                           `=---='

    ..................佛祖保佑, 永无BUG...................

*/

package tracer

import (
    "context"
    "github.com/Zzaniu/tool/ztracer"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/baggage"
    "go.opentelemetry.io/otel/codes"
    "go.opentelemetry.io/otel/trace"
    "google.golang.org/grpc"
    gcodes "google.golang.org/grpc/codes"
    "google.golang.org/grpc/metadata"
    "google.golang.org/grpc/status"
)

func ServerTraceInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
    handler grpc.UnaryHandler) (interface{}, error) {
    tr := ztracer.GetTrace()
    var md metadata.MD
    requestMetadata, ok := metadata.FromIncomingContext(ctx)
    if ok {
        md = requestMetadata.Copy()
    } else {
        md = metadata.MD{}
    }
    // 从 ctx 中提取 spanCtx, 提取 grpc 客户端通过 metadata 传过来的 TextMap
    bags, spanCtx := ztracer.Extract(ctx, otel.GetTextMapPropagator(), &md)
    ctx = baggage.ContextWithBaggage(ctx, bags)
    // ztracer.PeerFromCtx 获取客户端那边的一些 span 信息
    name, attr := ztracer.SpanInfo(info.FullMethod, ztracer.PeerFromCtx(ctx))
    // 从远程父 span 创建一个子 span
    ctx, span := tr.Start(trace.ContextWithRemoteSpanContext(ctx, spanCtx), name,
        trace.WithSpanKind(trace.SpanKindServer), trace.WithAttributes(attr...))
    defer span.End()

    ztracer.MessageReceived.Event(ctx, 1, req)

    resp, err := handler(ctx, req)
    if err != nil {
        s, ok := status.FromError(err)
        if ok {
            span.SetStatus(codes.Error, s.Message())
            span.SetAttributes(ztracer.StatusCodeAttr(s.Code()))
            ztracer.MessageSent.Event(ctx, 1, s.Proto())
        } else {
            span.SetStatus(codes.Error, err.Error())
        }
        return nil, err
    }

    span.SetAttributes(ztracer.StatusCodeAttr(gcodes.OK))
    ztracer.MessageSent.Event(ctx, 1, resp)

    return resp, nil
}
