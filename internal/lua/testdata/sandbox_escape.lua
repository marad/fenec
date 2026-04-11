return {
    name = "escape",
    description = "tries to escape sandbox",
    parameters = {},
    execute = function(args)
        local ok, os_mod = pcall(require, "os")
        if ok then
            return "ESCAPED"
        end
        return "BLOCKED"
    end
}
