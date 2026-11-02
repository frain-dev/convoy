import { CommonModule } from '@angular/common';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';

import { CardComponent } from 'src/app/components/card/card.component';
import { LabelComponent } from 'src/app/components/input/input.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { ToggleComponent } from 'src/app/components/toggle/toggle.component';
import { GeneralService } from 'src/app/services/general/general.service';

import { LoaderModule } from '../../../components/loader/loader.module';
import { AdminService } from '../admin.service';
import { OrganisationOverridesComponent } from './organisation-overrides.component';

const EMPTY_COPY = 'No feature flags available';

describe('OrganisationOverridesComponent', () => {
	let fixture: ComponentFixture<OrganisationOverridesComponent>;
	let component: OrganisationOverridesComponent;
	let answerFlags!: (flags: unknown[]) => void;

	beforeEach(async () => {
		await TestBed.configureTestingModule({
			declarations: [OrganisationOverridesComponent],
			imports: [CommonModule, ReactiveFormsModule, CardComponent, SelectComponent, ToggleComponent, TagComponent, LabelComponent, LoaderModule],
			providers: [
				{
					provide: AdminService,
					useValue: {
						// The flag read is held open and the overrides read answers at
						// once. That is the real ordering this panel has to survive:
						// selecting an organisation reveals the list before the flags
						// behind it are known.
						getAllFeatureFlags: () => new Promise(resolve => (answerFlags = flags => resolve({ data: flags }))),
						getAllOrganisations: () => Promise.resolve({ data: { content: [{ uid: 'org-1', name: 'Acme' }] } }),
						getOrganisationOverrides: () => Promise.resolve({ data: [] })
					}
				},
				{ provide: GeneralService, useValue: { showNotification: () => {} } }
			]
		}).compileComponents();

		fixture = TestBed.createComponent(OrganisationOverridesComponent);
		component = fixture.componentInstance;

		// The first pass runs ngOnInit, so ngOnInit is never called by hand here:
		// a second call would start a second pair of reads and overwrite the
		// resolver the test is holding.
		fixture.detectChanges();
	});

	afterEach(() => {
		fixture.destroy();
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

	// Selected through the component rather than assigned, so the overrides read
	// runs the way it does for an operator picking from the dropdown.
	async function selectOrganisation() {
		await component.selectOrganisation({ uid: 'org-1', name: 'Acme' });
		await settle();
	}

	it('does not claim the instance has no feature flags while the flag read is in flight', async () => {
		await selectOrganisation();

		paint();

		expect(text()).not.toContain(EMPTY_COPY);
	});

	it('claims it once the flag read answers with nothing', async () => {
		await selectOrganisation();
		answerFlags([]);
		await settle();

		paint();

		expect(text()).toContain(EMPTY_COPY);
	});

	it('never claims it once the flag read answers with a flag', async () => {
		await selectOrganisation();
		answerFlags([{ uid: 'flag-1', feature_key: 'ip-rules', enabled: true, allow_override: true }]);
		await settle();

		paint();

		expect(text()).not.toContain(EMPTY_COPY);
		expect(text()).toContain('IP Rules');
	});

	// A later read must answer for the present. Leaving the flag set would let a
	// finished read speak for one that has only just started.
	it('stops claiming anything while a re-read is in flight', async () => {
		await selectOrganisation();
		answerFlags([]);
		await settle();

		component.loadFeatureFlags();
		await settle();

		paint();

		expect(text()).not.toContain(EMPTY_COPY);
	});
});
