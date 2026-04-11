return {
    name = "word_count",
    description = "Count words in the given text",
    parameters = {
        { name = "text", type = "string", description = "Text to count words in", required = true }
    },
    execute = function(args)
        local text = args.text or ""
        local count = 0
        for _ in text:gmatch("%S+") do
            count = count + 1
        end
        return tostring(count)
    end
}
