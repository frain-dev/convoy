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

/**
 * @param {string} date an ISO date string
 * @example
 * // returns 'Nov 3, 1994'
 * toMMM_DD_YYYY("1994-11-03T00:00:00Z")
 */
export function toMMMDDYYYY(date: string) {
	return new Intl.DateTimeFormat('en-US', { dateStyle: 'medium' }).format(
		new Date(date),
	);
}
