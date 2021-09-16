# Set version
tag=$1
: > ./VERSION && echo $tag >  VERSION

# Commit version number & push
git add VERSION
git commit -m "Bump version to $tag"
git push origin

# Tag & Push.
git tag $tag
git push origin $tag
