# Build UI.
UIDIR="api/ui/build"

# Remove build folder
rm -rf $UIDIR

# Recreate build folder
mkdir $UIDIR

# Enter UI directory
cd ./web/ui/dashboard

# Install dependencies
npm ci

# Run production build
npm run build

# Copy build artifacts
cd ../../../
mv web/ui/dashboard/dist/* $UIDIR

# Build Binary
go build -o convoy ./cmd/*.go
