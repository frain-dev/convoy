export function getInitials(names: Array<string>) {
	const initials = names
		.filter(Boolean)
		.map(name => name[0])
		.join('');
	if (initials.length < 3) return initials;
	return initials[0] + initials[initials.length - 1];
}

export function truncateProjectName(name: string) {
	const formattedName = name.substring(0, 27);
	if (name > formattedName) return formattedName + '...';
	return formattedName;
}
