# Swagger 2.0 cannot describe a response selected by an opt-in query parameter.
# The handler marks the operation; reuse its generated array/item schema rather
# than duplicate DeliveryAttempt fields here. Keep this transform idempotent.
.paths |= with_entries(
  .value |= with_entries(
    if (.value | type) == "object" and .value["x-convoy-paginated-response"] == true then
      .value.responses["200"].content["application/json"].schema.allOf[1].properties.data |= (
        if has("oneOf") then .
        elif .type == "array" and .items != null then
          {oneOf: [., {
            type: "object",
            required: ["content", "next_cursor"],
            properties: {
              content: .,
              next_cursor: {
                type: "string",
                description: "Pass as before to fetch older attempts. Empty on the last page."
              }
            }
          }]}
        else error("Paginated response must have an array data schema") end
      )
    else . end
  )
)
