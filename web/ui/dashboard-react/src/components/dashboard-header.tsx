import { useEffect, useState } from 'react';
import {
	SidebarIcon,
	ChevronDown,
	CirclePlusIcon,
	SettingsIcon,
} from 'lucide-react';
import * as authService from '@/services/auth.service';
import * as organisationsService from '@/services/organisations.service';

import { Button } from '@/components/ui/button';
import { Avatar, AvatarFallback } from '@/components/ui/avatar';
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuGroup,
	DropdownMenuItem,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { useSidebar } from '@/components/ui/sidebar';

import convoyLogo from '../../assets/svg/logo.svg';
import userProfileIcon from '../../assets/svg/user-icon.svg';

import * as transform from '@/lib/pipes';
import type { Organisation } from '@/models/organisation.model';
import { useNavigate } from '@tanstack/react-router';

export function DashboardHeader() {
	const { toggleSidebar } = useSidebar();
	const navigate = useNavigate({ from: '/projects' });
	// TODO move this to a hook for organisations
	const [organisations, setOrganisations] = useState<Array<Organisation>>([]);
	const [selectedOrganisation, setSelectedOrganisation] =
		useState<Organisation | null>(null);
	const [isLoadingOrganisations, setIsLoadingOrganisations] = useState(false);
	const [currentUser] = useState(authService.getCachedAuthProfile());

	useEffect(() => {
		setIsLoadingOrganisations(true);
		organisationsService
			.getOrganisations({ refresh: true })
			.then(res => setOrganisations(res.content))
			.catch(console.error)
			.finally(() => {
				setIsLoadingOrganisations(false);
				setSelectedOrganisation(organisationsService.getCachedOrganisation());
			});

		return () => {};
	}, []);

	function setCurrentOrganisation(org: Organisation) {
		console.log(
			`todo: set ${org.name} with id = ${org.uid} as current organisation`,
		);
	}

	return (
		<header className=" sticky top-0 z-50 border-b bg-background px-6">
			<div className="max-w-[1440px] flex w-full items-center justify-between mx-auto">
				<div className="flex items-center justify-between py-3 h-[60px] w-[17rem]">
					<a href="/" className="inline-block" rel="noreferrer">
						<img src={convoyLogo} alt="Convoy" className="w-[100px]" />
					</a>
					<div className="flex items-center ">
						<Button
							className="h-8 w-8"
							variant="ghost"
							size="icon"
							onClick={toggleSidebar}
							title="CTRL + B"
						>
							<SidebarIcon />
						</Button>
					</div>
				</div>

				<nav>
					<ul className="flex items-center justify-end">
						{/* TODO use a skeleton here for loading */}
						{isLoadingOrganisations ? (
							<p className="text-xs">Skeleton</p>
						) : selectedOrganisation ? (
							<li className="mr-3">
								<DropdownMenu>
									<DropdownMenuTrigger asChild>
										<Button
											variant="ghost"
											className="bg-neutral-3 shadow-none px-5 flex justify-start items-center focus-visible:ring-0"
										>
											<Avatar className="rounded-[100%] w-6 h-6">
												<AvatarFallback className="bg-new.primary-600 text-white-100 text-[10px]">
													{transform.getInitials(
														selectedOrganisation.name.split(' '),
													)}
												</AvatarFallback>
											</Avatar>
											<span className="text-xs block px-1">
												{selectedOrganisation?.name}
											</span>
											<ChevronDown />
										</Button>
									</DropdownMenuTrigger>

									<DropdownMenuContent align="end" className="w-64">
										<DropdownMenuGroup className="py-1">
											<DropdownMenuItem className="focus:bg-transparent text-xs font-semibold focus:text-neutral-11 text-neutral-11 truncate py-1 hover:bg-transparent hover:cursor-default">
												Your organisations ({organisations.length})
											</DropdownMenuItem>
										</DropdownMenuGroup>

										<DropdownMenuSeparator />

										<DropdownMenuGroup>
											<DropdownMenuItem
												className="gap-0 focus:bg-transparent text-xs text-neutral-11 p-3 py-1 hover:cursor-pointer ring-1 flex justify-start items-center"
												onClick={() => navigate({ to: '/projects' })}
											>
												<Button
													variant={'ghost'}
													className="p-0 gap-0 flex justify-start w-[85%] hover:bg-transparent"
												>
													<Avatar className="rounded-[100%] w-6 h-6 text-xs mr-2">
														<AvatarFallback className="bg-new.primary-600 text-white-100 text-[10px]">
															{transform.getInitials(
																selectedOrganisation.name.split(' '),
															)}
														</AvatarFallback>
													</Avatar>
													<span className="text-xs text-start block truncate w-3/4">
														{selectedOrganisation.name}
													</span>
												</Button>
												<a
													href={`/organisations/${selectedOrganisation.uid}/settings`}
													className="block p-2 bg-new.primary-25 rounded-8px transition-colors"
												>
													<SettingsIcon
														size={18}
														className="stroke-neutral-9"
													/>
												</a>
											</DropdownMenuItem>
										</DropdownMenuGroup>

										<DropdownMenuSeparator />

										<DropdownMenuGroup>
											<ul>
												{organisations
													.filter(org => org.uid != selectedOrganisation.uid)
													.toSorted((oA, oB) => {
														if (oA.name < oB.name) return -1;
														if (oA.name > oB.name) return 1;
														return 0;
													})
													.map(org => {
														return (
															<DropdownMenuItem
																key={org.uid}
																className="gap-0 text-xs text-neutral-11 p-3 py-1 hover:cursor-pointer flex justify-start items-center hover:bg-neutral-3"
																onClick={() => setCurrentOrganisation(org)}
															>
																<Button
																	variant={'ghost'}
																	className="p-0 gap-0 flex justify-start w-[85%] hover:bg-transparent"
																>
																	<Avatar className="rounded-[100%] w-6 h-6 mr-2">
																		<AvatarFallback className="bg-new.primary-600 text-white-100 text-[10px]">
																			{transform.getInitials(
																				org.name.split(' '),
																			)}
																		</AvatarFallback>
																	</Avatar>
																	<span className="text-xs text-start block truncate w-3/4">
																		{org.name}
																	</span>
																</Button>
															</DropdownMenuItem>
														);
													})}
											</ul>
										</DropdownMenuGroup>

										<DropdownMenuSeparator />
										<DropdownMenuItem className="flex justify-center items-center hover:cursor-pointer hover:bg-neutral-3 py-3">
											<div className="flex items-center justify-center">
												<CirclePlusIcon
													className="stroke-new.primary-400 mr-2"
													size={20}
												/>
												<span className="block text-new.primary-400 text-xs ">
													Add {organisations.length == 0 ? 'an' : 'another'}{' '}
													organisation
												</span>
											</div>
										</DropdownMenuItem>
									</DropdownMenuContent>
								</DropdownMenu>
							</li>
						) : null}

						{currentUser ? (
							<li>
								<DropdownMenu>
									<DropdownMenuTrigger asChild>
										<Button
											variant="ghost"
											className="bg-neutral-3 shadow-none focus-visible:ring-0"
										>
											<img src={userProfileIcon} alt="user profile dropdown" />
										</Button>
									</DropdownMenuTrigger>
									<DropdownMenuContent className="w-64" align="end">
										<DropdownMenuGroup className="pt-2 pb-2 px-3">
											<div className="flex flex-col">
												<p className="capitalize text-sm font-medium truncate p-0 hover:bg-transparent hover:cursor-default">
													{currentUser.first_name || ''}{' '}
													{currentUser.last_name || ''}
												</p>
												<p className="text-neutral-11 text-xs p-0 mt-1 hover:bg-transparent hover:cursor-default">
													{currentUser.email}
												</p>
											</div>
										</DropdownMenuGroup>
										<DropdownMenuSeparator />
										<DropdownMenuItem className="hover:cursor-pointer hover:bg-neutral-3 pl-3">
											<a
												href="/user-settings"
												className="block w-full text-xs text-neutral-11"
											>
												My account
											</a>
										</DropdownMenuItem>
										<DropdownMenuSeparator />
										<DropdownMenuItem
											className="hover:cursor-pointer hover:bg-neutral-3 pl-3"
											onClick={authService.logUserOut}
										>
											<span className="text-destructive hover:text-destructive text-xs">
												Logout
											</span>
										</DropdownMenuItem>
									</DropdownMenuContent>
								</DropdownMenu>
							</li>
						) : (
							<p className="text-xs">Skeleton</p>
						)}
					</ul>
				</nav>
			</div>
		</header>
	);
}
