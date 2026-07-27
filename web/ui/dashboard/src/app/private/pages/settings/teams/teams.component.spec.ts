import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FormBuilder } from '@angular/forms';
import { ActivatedRoute } from '@angular/router';
import { RouterTestingModule } from '@angular/router/testing';
import { GeneralService } from 'src/app/services/general/general.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { RbacService } from 'src/app/services/rbac/rbac.service';
import { PrivateService } from '../../../private.service';
import { TeamsComponent } from './teams.component';
import { TeamsService } from './teams.service';

describe('TeamsComponent', () => {
	let component: TeamsComponent;
	let fixture: ComponentFixture<TeamsComponent>;

	beforeEach(async () => {
		await TestBed.configureTestingModule({
			declarations: [TeamsComponent],
			imports: [RouterTestingModule],
			providers: [
				FormBuilder,
				{ provide: ActivatedRoute, useValue: { snapshot: { queryParams: {} } } },
				{ provide: GeneralService, useValue: { showNotification: () => {} } },
				{ provide: TeamsService, useValue: {} },
				{ provide: PrivateService, useValue: { getUserProfile: {}, getTeamMembers: jasmine.createSpy('getTeamMembers') } },
				{ provide: LicensesService, useValue: { isMultiUserMode: () => true } },
				{ provide: RbacService, useValue: { userCanAccess: async () => true } }
			]
		})
			.overrideComponent(TeamsComponent, { set: { template: '' } })
			.compileComponents();
	});

	beforeEach(() => {
		fixture = TestBed.createComponent(TeamsComponent);
		component = fixture.componentInstance;
	});

	it('should create', () => {
		expect(component).toBeTruthy();
	});

	it('defaults invite and member role to organisation_admin', () => {
		expect(component.inviteUserForm.get('role.type')?.value).toBe('organisation_admin');
		expect(component.memberForm.get('role.type')?.value).toBe('organisation_admin');
	});

	it('only exposes valid invite roles', () => {
		expect(component.roles.map((role) => role.uid)).toEqual(['organisation_admin', 'billing_admin', 'project_admin', 'project_viewer']);
	});

	it('resets invite role to organisation_admin when closing the modal', () => {
		component.teamsDialog = { nativeElement: { close: () => {} } } as TeamsComponent['teamsDialog'];
		component.inviteUserForm.patchValue({ role: { type: 'project_viewer' } });
		component.closeInviteModal();

		expect(component.inviteUserForm.get('role.type')?.value).toBe('organisation_admin');
	});
});
