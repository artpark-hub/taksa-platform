NATS_TOOL="/home/kavya/go/bin/nats"

echo "🏭 STARTING FACTORY SIMULATOR..."
echo "Press [CTRL+C] to stop."

while true; do
  # Generate random temp between 50-100
  TEMP=$(( ( RANDOM % 50 ) + 50 ))
  
  # Create JSON Payload
  PAYLOAD="{\"equipment_id\": \"CNC-001\", \"event_type\": \"STATUS_UPDATE\", \"work_order_id\": \"WO-999\", \"operator_id\": \"OP-KAVYA\", \"temperature\": $TEMP}"
  
  # Publish to NATS
  $NATS_TOOL pub $SUBJECT "$PAYLOAD"
  
  echo "--- 📤 Sent Update: Temp $TEMP ---"
  sleep 5
done
