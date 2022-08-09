docker exec mongo1 mongosh --eval "rs.initiate({
   _id: \"myReplicaSet\",
   members: [
     {_id: 0, host: \"mongo1\"},
     {_id: 1, host: \"mongo2\"},
     {_id: 2, host: \"mongo3\"}
   ]
})"
