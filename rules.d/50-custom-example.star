# Starlark example: custom routing logic
# Priority derived from filename: 50-custom-example.star → priority 50
#
# Available ctx fields:
#   last_user_message, all_user_messages, message_count,
#   total_chars, approx_tokens, has_tool_calls, model_ids
#   log(msg) — debug log

def rule(payload, ctx):
    msgs = ctx.get("all_user_messages", [])
    last = ctx.get("last_user_message", "")
    tokens = ctx.get("approx_tokens", 0)
    count = ctx.get("message_count", 0)

    # If conversation is long and has TODO marker → DeepSeek
    if count > 5 and "TODO" in last:
        return {
            "match": True,
            "target": "deepseek",
            "reason": "long conversation with TODO marker",
        }

    # If very short and few messages → Haiku
    if tokens < 20 and count <= 3:
        return {
            "match": True,
            "target": "haiku",
            "reason": "short conversation, use fast model",
        }

    return {"match": False}