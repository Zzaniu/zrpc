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
    "go.opentelemetry.io/otel/codes"
    "go.opentelemetry.io/otel/trace"
    "google.golang.org/grpc"
    gcodes "google.golang.org/grpc/codes"
    "google.golang.org/grpc/metadata"
    "google.golang.org/grpc/status"
)

// ClientTraceInterceptor 链路追踪拦截器
func ClientTraceInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn,
    invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
    tr := ztracer.GetTrace()
    var md metadata.MD
    // 从 ctx 中提取 metadata
    requestMetadata, ok := metadata.FromOutgoingContext(ctx)
    if ok {
        md = requestMetadata.Copy()
    } else {
        md = metadata.MD{}
    }
    name, attr := ztracer.SpanInfo(method, cc.Target())
    ctx, span := tr.Start(ctx, name, trace.WithSpanKind(trace.SpanKindClient),
        trace.WithAttributes(attr...))
    defer span.End()

    // Inject: 注入一个SpanContext到一个载体, Extract: 提取一个SpanContext从载体
    // 将元数据 trace parent 注入 md
    ztracer.Inject(ctx, otel.GetTextMapPropagator(), &md)
    // 然后再注入到 ctx, 链路就成了
    ctx = metadata.NewOutgoingContext(ctx, md)

    ztracer.MessageSent.Event(ctx, 1, req)
    ztracer.MessageReceived.Event(ctx, 1, reply)

    // 调用远端 rpc 服务
    if err := invoker(ctx, method, req, reply, cc, opts...); err != nil {
        s, ok := status.FromError(err)
        if ok {
            // 设置错误信息
            span.SetStatus(codes.Error, s.Message())
            // 设置 grpc 返回状态码
            span.SetAttributes(ztracer.StatusCodeAttr(s.Code()))
        } else {
            span.SetStatus(codes.Error, err.Error())
        }
        return err
    }

    // 说明返回成功
    span.SetAttributes(ztracer.StatusCodeAttr(gcodes.OK))

    return nil
}
