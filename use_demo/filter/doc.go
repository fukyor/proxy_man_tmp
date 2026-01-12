/*
了解责任链模式，同一个注册handler中所有的条件函数(bool)组成一条责任链，每个条件是&&关系。必须全部满足才会执行handler。所以如果要多次拦截请求(保底)，我们需要注册多个handler。注册好的handler放到[]reqHandlers中。如下：
reqHandlers = [
    // 第1个处理器（完整的包装）
    FuncReqHandler(func(r, ctx) {
        // 这个处理器的条件检查
        for _, cond := range [cond1, cond2] {
            if !cond.HandleReq(r, ctx) {
                return r, nil
            }
        }
        // 这个处理器的用户逻辑
        return userFunc1(r, ctx)
    }),
    
    // 第2个处理器（另一个完整的包装）
    FuncReqHandler(func(r, ctx) {
        // 这个处理器的条件检查
        for _, cond := range [cond3] {
            if !cond.HandleReq(r, ctx) {
                return r, nil
            }
        }
        // 这个处理器的用户逻辑
        return userFunc2(r, ctx)
    }),
    
    // ... 更多处理器
]
*/
package main