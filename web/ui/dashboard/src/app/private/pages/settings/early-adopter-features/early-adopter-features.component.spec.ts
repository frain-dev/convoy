import { CommonModule } from '@angular/common';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute } from '@angular/router';

import { ToggleComponent } from 'src/app/components/toggle/toggle.component';
import { GeneralService } from 'src/app/services/general/general.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';
import { RbacService } from 'src/app/services/rbac/rbac.service';

import { LoaderModule } from '../../../components/loader/loader.module';
import { PermissionDirective } from '../../../components/permission/permission.directive';
import { SettingsService } from '../settings.service';
import { EarlyAdopterFeaturesComponent } from './early-adopter-features.component';

const EMPTY_COPY = 'No early adopter features available';

describe('EarlyAdopterFeaturesComponent', () => {
	let fixture: ComponentFixture<EarlyAdopterFeaturesComponent>;
	let answerRole!: () => void;
	let answerFeatures!: (features: unknown[]) => void;
	let failFeatures!: () => void;
	let notifications: Array<{ style: string; message: string }>;

	beforeEach(async () => {
		localStorage.setItem('CONVOY_ORG', JSON.stringify({ uid: 'org-1' }));
		notifications = [];

		await TestBed.configureTestingModule({
			declarations: [EarlyAdopterFeaturesComponent],
			imports: [CommonModule, ToggleComponent, PermissionDirective, LoaderModule],
			providers: [
				{
					provide: SettingsService,
					useValue: {
						// Held open so a test can paint the page while the read is in
						// flight. The role read lands first, which is what makes the
						// empty state reachable before this one has answered anything.
						getEarlyAdopterFeatures: () =>
							new Promise((resolve, reject) => {
								answerFeatures = features => resolve({ data: features });
								failFeatures = () => reject(new Error('network down'));
							}),
						updateOrganisationFeatureFlags: () => new Promise(() => {})
					}
				},
				{
					provide: RbacService,
					useValue: {
						getUserRole: () => new Promise(resolve => (answerRole = () => resolve('ORGANISATION_ADMIN'))),
						userPermission: () => Promise.resolve(['Organisations|MANAGE'])
					}
				},
				{ provide: LicensesService, useValue: { hasLicense: () => true } },
				{ provide: GeneralService, useValue: { showNotification: (details: { style: string; message: string }) => notifications.push(details) } },
				{ provide: ActivatedRoute, useValue: { snapshot: { queryParams: {} } } }
			]
		}).compileComponents();

		fixture = TestBed.createComponent(EarlyAdopterFeaturesComponent);

		// The first pass runs ngOnInit, so ngOnInit is never called by hand here:
		// a second call would start a second pair of reads and overwrite the
		// resolvers the test is holding.
		fixture.detectChanges();
	});

	afterEach(() => {
		fixture.destroy();
		localStorage.removeItem('CONVOY_ORG');
	});

	// The component's own detector, not fixture.detectChanges: these tests move
	// state between passes on purpose, which is what the fixture's verification
	// pass exists to reject.
	function paint() {
		fixture.changeDetectorRef.detectChanges();
	}

	function text(): string {
		return (fixture.nativeElement as HTMLElement).textContent ?? '';
	}

	async function settle() {
		for (let turn = 0; turn < 5; turn++) await Promise.resolve();
	}

	it('does not claim the organisation has no features before the role read lands', () => {
		paint();

		expect(text()).not.toContain(EMPTY_COPY);
	});

	it('does not claim it while the feature read is in flight', async () => {
		answerRole();
		await settle();

		paint();

		expect(text()).not.toContain(EMPTY_COPY);
	});

	it('claims it once the feature read answers with nothing', async () => {
		answerRole();
		await settle();
		answerFeatures([]);
		await settle();

		paint();

		expect(text()).toContain(EMPTY_COPY);
	});

	it('never claims it when the feature read answers with a feature', async () => {
		answerRole();
		await settle();
		answerFeatures([{ key: 'mtls', name: 'Mutual TLS', description: 'Client certificates', enabled: false }]);
		await settle();

		paint();

		expect(text()).not.toContain(EMPTY_COPY);
		expect(text()).toContain('Mutual TLS');
	});

	// The empty state is the same copy either way, so a failed read is
	// indistinguishable from an empty organisation unless it is reported. The
	// whole array is asserted so one failure cannot toast twice.
	it('reports a failed feature read', async () => {
		answerRole();
		await settle();
		failFeatures();
		await settle();

		expect(notifications).toEqual([{ style: 'error', message: 'Failed to load early adopter features' }]);
	});

	it('reports nothing when the feature read answers', async () => {
		answerRole();
		await settle();
		answerFeatures([]);
		await settle();

		expect(notifications).toEqual([]);
	});
});
