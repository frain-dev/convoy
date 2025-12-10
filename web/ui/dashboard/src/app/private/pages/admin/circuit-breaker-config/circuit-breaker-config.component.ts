import { Component, OnInit } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { AdminService } from '../admin.service';
import { GeneralService } from 'src/app/services/general/general.service';

interface Organisation {
	uid: string;
	name: string;
	created_at?: string;
	updated_at?: string;
}

interface Project {
	uid: string;
	name: string;
	organisation_id?: string;
	created_at?: string;
	updated_at?: string;
}

interface CircuitBreakerConfig {
	sample_rate?: number;
	error_timeout?: number;
	failure_threshold?: number;
	success_threshold?: number;
	observability_window?: number;
	minimum_request_count?: number;
	consecutive_failure_threshold?: number;
}

@Component({
	selector: 'app-circuit-breaker-config',
	templateUrl: './circuit-breaker-config.component.html',
	styleUrls: ['./circuit-breaker-config.component.scss']
})
export class CircuitBreakerConfigComponent implements OnInit {
	// Organization-related properties
	organisations: Organisation[] = [];
	selectedOrganisation: Organisation | null = null;
	isLoadingOrganisations = false;
	organisationForm: FormGroup;
	organisationSearchTerm = '';
	private searchTimeout: any;

	// Project-related properties
	projects: Project[] = [];
	selectedProject: Project | null = null;
	isLoadingProjects = false;
	projectForm: FormGroup;

	// Circuit breaker configuration
	circuitBreakerForm: FormGroup | null = null;
	circuitBreakerConfig: CircuitBreakerConfig | null = null;
	isLoadingCircuitBreakerConfig = false;
	isSavingCircuitBreakerConfig = false;

	constructor(
		private adminService: AdminService,
		private generalService: GeneralService,
		private formBuilder: FormBuilder
	) {
		this.organisationForm = this.formBuilder.group({
			organisation: [null]
		});
		this.projectForm = this.formBuilder.group({
			project: [null]
		});
	}

	async ngOnInit() {
		await this.loadOrganisations();
	}

	// Organization methods
	async loadOrganisations(searchTerm?: string) {
		this.isLoadingOrganisations = true;
		try {
			const response = await this.adminService.getAllOrganisations({ 
				page: 1, 
				perPage: 1000,
				search: searchTerm || ''
			});
			this.organisations = response.data?.content || [];
		} catch (error) {
			console.error('Error loading organisations:', error);
			this.generalService.showNotification({ style: 'error', message: 'Failed to load organisations' });
		} finally {
			this.isLoadingOrganisations = false;
		}
	}

	filterOrganisations(searchTerm: string) {
		this.organisationSearchTerm = searchTerm;
		
		if (this.searchTimeout) {
			clearTimeout(this.searchTimeout);
		}

		this.searchTimeout = setTimeout(() => {
			this.loadOrganisations(searchTerm.trim());
		}, 500);
	}

	async selectOrganisation(org: Organisation) {
		if (!org || !org.uid) {
			console.error('Invalid organisation:', org);
			return;
		}
		this.selectedOrganisation = org;
		this.selectedProject = null;
		this.projects = [];
		// Clear project form - reset to ensure select component properly clears
		this.projectForm.reset({ project: null });
		this.circuitBreakerForm = null;
		this.circuitBreakerConfig = null;
		await this.loadProjects(org.uid);
	}

	getOrganisationOptions(): Array<{ uid: string; name: string }> {
		return this.organisations.map(org => ({ 
			uid: org.uid, 
			name: `${org.name} (${org.uid})` 
		}));
	}

	onOrganisationSelected(selectedOrg: any) {
		const orgUid = typeof selectedOrg === 'string' ? selectedOrg : selectedOrg?.uid;
		if (!orgUid) return;
		
		const org = this.organisations.find(o => o.uid === orgUid);
		if (org) {
			// Update form control to the selected option object so dropdown shows the name
			this.organisationForm.patchValue({ organisation: { uid: org.uid, name: `${org.name} (${org.uid})` } });
			this.selectOrganisation(org);
		}
	}

	// Project methods
	async loadProjects(orgID: string) {
		this.isLoadingProjects = true;
		try {
			const response = await this.adminService.getOrganisationProjects(orgID);
			this.projects = response.data?.content || response.data || [];
		} catch (error) {
			console.error('Error loading projects:', error);
			this.generalService.showNotification({ style: 'error', message: 'Failed to load projects' });
			this.projects = [];
		} finally {
			this.isLoadingProjects = false;
		}
	}

	async selectProject(project: Project) {
		if (!project || !project.uid) {
			console.error('Invalid project:', project);
			return;
		}
		this.selectedProject = project;
		await this.loadCircuitBreakerConfig(project.uid);
	}

	getProjectOptions(): Array<{ uid: string; name: string }> {
		return this.projects.map(project => ({ 
			uid: project.uid, 
			name: `${project.name} (${project.uid})` 
		}));
	}

	onProjectSelected(selectedProject: any) {
		const projectUid = typeof selectedProject === 'string' ? selectedProject : selectedProject?.uid;
		if (!projectUid) return;
		
		const project = this.projects.find(p => p.uid === projectUid);
		if (project) {
			// Update form control to the selected option object so dropdown shows the name
			this.projectForm.patchValue({ project: { uid: project.uid, name: `${project.name} (${project.uid})` } });
			this.selectProject(project);
		}
	}

	// Circuit breaker configuration methods
	async loadCircuitBreakerConfig(projectID: string) {
		this.isLoadingCircuitBreakerConfig = true;
		try {
			const response = await this.adminService.getProjectCircuitBreakerConfig(projectID);
			this.circuitBreakerConfig = response.data || null;
			this.initializeCircuitBreakerForm();
		} catch (error) {
			console.error('Error loading circuit breaker config:', error);
			// If endpoint doesn't exist yet, use defaults
			this.circuitBreakerConfig = null;
			this.initializeCircuitBreakerForm();
		} finally {
			this.isLoadingCircuitBreakerConfig = false;
		}
	}

	initializeCircuitBreakerForm() {
		const config = this.circuitBreakerConfig || {};
		this.circuitBreakerForm = this.formBuilder.group({
			sample_rate: [
				config.sample_rate || null,
				[Validators.required, Validators.min(1)]
			],
			error_timeout: [
				config.error_timeout || null,
				[Validators.required, Validators.min(1)]
			],
			failure_threshold: [
				config.failure_threshold || null,
				[Validators.required, Validators.min(0), Validators.max(100)]
			],
			success_threshold: [
				config.success_threshold || null,
				[Validators.required, Validators.min(0), Validators.max(100)]
			],
			observability_window: [
				config.observability_window || null,
				[Validators.required, Validators.min(1)]
			],
			minimum_request_count: [
				config.minimum_request_count || null,
				[Validators.required, Validators.min(0)]
			],
			consecutive_failure_threshold: [
				config.consecutive_failure_threshold || null,
				[Validators.required, Validators.min(0)]
			]
		});
	}

	resetCircuitBreakerForm() {
		if (this.circuitBreakerForm) {
			this.initializeCircuitBreakerForm();
		}
	}

	async saveCircuitBreakerConfig() {
		if (!this.selectedProject || !this.circuitBreakerForm || !this.circuitBreakerForm.valid) {
			return;
		}

		this.isSavingCircuitBreakerConfig = true;
		try {
			const formValue = this.circuitBreakerForm.value;
			const response = await this.adminService.updateProjectCircuitBreakerConfig(
				this.selectedProject.uid,
				{
					sample_rate: formValue.sample_rate,
					error_timeout: formValue.error_timeout,
					failure_threshold: formValue.failure_threshold,
					success_threshold: formValue.success_threshold,
					observability_window: formValue.observability_window,
					minimum_request_count: formValue.minimum_request_count,
					consecutive_failure_threshold: formValue.consecutive_failure_threshold
				}
			);
			
			this.circuitBreakerConfig = { ...this.circuitBreakerConfig, ...response.data };
			this.circuitBreakerForm.markAsPristine();
			this.generalService.showNotification({ style: 'success', message: 'Circuit breaker configuration updated successfully' });
		} catch (error) {
			console.error('Error saving circuit breaker config:', error);
			this.generalService.showNotification({ style: 'error', message: 'Failed to update circuit breaker configuration' });
		} finally {
			this.isSavingCircuitBreakerConfig = false;
		}
	}
}
