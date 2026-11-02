import { ComponentFixture, TestBed } from '@angular/core/testing';
import { CommonModule } from '@angular/common';
import { ReactiveFormsModule } from '@angular/forms';
import { TablePartitionsComponent } from './table-partitions.component';
import { RunCardComponent } from '../runs/run-card.component';
import { SelectComponent } from 'src/app/components/select/select.component';
import { TagComponent } from 'src/app/components/tag/tag.component';
import { StatusColorModule } from 'src/app/pipes/status-color/status-color.module';
import { LabelComponent, InputFieldDirective, InputErrorComponent, InputDirective } from 'src/app/components/input/input.component';
import { AdminService } from '../admin.service';
import { GeneralService } from 'src/app/services/general/general.service';
import { LicensesService } from 'src/app/services/licenses/licenses.service';

// events_search is the shape that matters: converted already, so the only
// operation left for it is the destructive one.
const TABLE_STATES = [
	{ name: 'events', partitioned: false, adopted: false },
	{ name: 'events_search', partitioned: true, adopted: false },
	{ name: 'event_deliveries', partitioned: false, adopted: false },
	{ name: 'delivery_attempts', partitioned: false, adopted: false }
];

describe('TablePartitionsComponent', () => {
	let fixture: ComponentFixture<TablePartitionsComponent>;
	let component: TablePartitionsComponent;

	beforeEach(async () => {
		await TestBed.configureTestingModule({
			declarations: [TablePartitionsComponent, RunCardComponent],
			imports: [CommonModule, ReactiveFormsModule, SelectComponent, TagComponent, StatusColorModule, LabelComponent, InputFieldDirective, InputErrorComponent, InputDirective],
			providers: [
				// Left pending on purpose. ngOnInit still runs, and a stub that
				// resolves would land mid-pass and re-render the page from a
				// half-loaded state while these tests are reading the DOM.
				{
					provide: AdminService,
					useValue: {
						listPartitionTables: () => new Promise(() => {}),
						listPartitionRuns: () => new Promise(() => {}),
						startPartitionRun: () => new Promise(() => {})
					}
				},
				{ provide: GeneralService, useValue: { showNotification: () => {} } },
				{ provide: LicensesService, useValue: { loadAllLicenses: () => new Promise(() => {}), hasInstanceLicense: () => true } }
			]
		}).compileComponents();

		fixture = TestBed.createComponent(TablePartitionsComponent);
		component = fixture.componentInstance;

		// Seeded rather than awaited through ngOnInit: the licence and table-state
		// fetches resolve across change detection passes, and the resulting
		// half-loaded renders are not what these tests are about.
		component.hasRetentionLicense = true;
		component.tableStates = TABLE_STATES;
		component.tableStatesKnown = true;
		component.applyOperations();

		// Angular schedules the render, as it does for a real click. Driving
		// detectChanges by hand instead leaves the view unrefreshed, because
		// nothing in the test marked it dirty, and then the page looks stale
		// whether or not the component is.
		fixture.autoDetectChanges();
		await fixture.whenStable();
	});

	function fieldLabelled(label: string): HTMLElement {
		const fields = Array.from(fixture.nativeElement.querySelectorAll('convoy-input-field')) as HTMLElement[];
		return fields.find(candidate => candidate.querySelector('label')?.textContent?.trim() === label)!;
	}

	// The operation as an operator reads it off the page, not as the form holds it.
	function renderedOperation(): string {
		return fieldLabelled('Operation').querySelector('div[convoy-input] span')?.textContent?.trim().toLowerCase() ?? '';
	}

	// Clicked rather than called: selectOption on its own changes no binding
	// Angular knows about, so the page would keep the markup it first rendered
	// and every assertion below would pass on a stale DOM.
	async function chooseTable(name: string) {
		const field = fieldLabelled('Table');
		(field.querySelector('[dropdowntrigger]') as HTMLElement).click();
		await fixture.whenStable();

		const option = Array.from(field.querySelectorAll('button')).find(candidate => candidate.textContent?.trim() === name);
		(option as HTMLElement).click();
		await fixture.whenStable();
	}

	it('shows the operation the button will run on a table that is not partitioned', () => {
		expect(component.selectedTable).toBe('events');
		expect(component.resolvedOperation).toBe('partition');
		expect(renderedOperation()).toBe('partition');
	});

	it('shows the operation the button will run after switching to a partitioned table', async () => {
		await chooseTable('events_search');

		expect(component.resolvedOperation).toBe('unpartition');
		// The label an operator reads must not disagree with the destructive
		// operation the button starts.
		expect(renderedOperation()).toBe('unpartition');
	});

	it('shows the operation the button will run after switching back to a heap', async () => {
		await chooseTable('events_search');
		await chooseTable('delivery_attempts');

		expect(component.resolvedOperation).toBe('partition');
		expect(renderedOperation()).toBe('partition');
	});

	// The confirmation is typed correctly in both halves, so the run list is the
	// only thing deciding. An index rebuild or a conversion started from a shell
	// takes the same instance-wide slot, so a history read that failed leaves the
	// slot unknown, and an empty list from an earlier poll is not evidence it is
	// free.
	it('does not offer a start while the run history is unknown', () => {
		component.partitionForm.patchValue({ confirmation: 'events' });

		component.runsKnown = true;
		expect(component.canStart).toBeTrue();

		component.runsKnown = false;
		expect(component.canStart).toBeFalse();
	});

	// Rebuilds share the slot and the API response, but this page only shows
	// conversions. A rebuild-only history must not look like a past conversion.
	it('lists conversion runs and hides rebuilds', () => {
		component.runs = [
			{
				uid: 'run-rebuild',
				table_name: 'events',
				operation: 'rebuild_index',
				index_name: 'idx_events_source_id',
				status: 'completed',
				phase: null,
				steps: null,
				error: null,
				triggered_by: 'user-1',
				started_at: '2026-08-19T09:00:00Z',
				updated_at: '2026-08-19T09:01:00Z',
				completed_at: '2026-08-19T09:01:00Z'
			},
			{
				uid: 'run-partition',
				table_name: 'events',
				operation: 'partition',
				index_name: null,
				status: 'completed',
				phase: null,
				steps: null,
				error: null,
				triggered_by: 'user-1',
				started_at: '2026-08-18T09:00:00Z',
				updated_at: '2026-08-18T09:01:00Z',
				completed_at: '2026-08-18T09:01:00Z'
			}
		];
		component.runsKnown = true;

		expect(component.conversionRuns.map(run => run.uid)).toEqual(['run-partition']);
	});

	it('treats a rebuild-only history as no conversions yet', () => {
		component.runs = [
			{
				uid: 'run-rebuild',
				table_name: 'events',
				operation: 'rebuild_index',
				index_name: 'idx_events_source_id',
				status: 'completed',
				phase: null,
				steps: null,
				error: null,
				triggered_by: 'user-1',
				started_at: '2026-08-19T09:00:00Z',
				updated_at: '2026-08-19T09:01:00Z',
				completed_at: '2026-08-19T09:01:00Z'
			}
		];
		component.runsKnown = true;

		expect(component.conversionRuns.length).toBe(0);
	});
});

// Separate fixture: the suite above seeds the licence so it can reach the form,
// which is the one thing these tests must not do. Here the licence read is held
// open on purpose, because the flash happens in exactly that window.
describe('TablePartitionsComponent license gate', () => {
	const UNLICENSED_COPY = 'Partitioning requires a license key';

	let fixture: ComponentFixture<TablePartitionsComponent>;
	let licensed: boolean;
	let answerLicense!: () => void;

	beforeEach(async () => {
		licensed = true;

		await TestBed.configureTestingModule({
			declarations: [TablePartitionsComponent, RunCardComponent],
			imports: [CommonModule, ReactiveFormsModule, SelectComponent, TagComponent, StatusColorModule, LabelComponent, InputFieldDirective, InputErrorComponent, InputDirective],
			providers: [
				{
					provide: AdminService,
					useValue: {
						listPartitionTables: () => new Promise(() => {}),
						listPartitionRuns: () => new Promise(() => {}),
						startPartitionRun: () => new Promise(() => {})
					}
				},
				{ provide: GeneralService, useValue: { showNotification: () => {} } },
				{
					provide: LicensesService,
					useValue: {
						loadAllLicenses: () => new Promise<void>(resolve => (answerLicense = resolve)),
						hasInstanceLicense: () => licensed
					}
				}
			]
		}).compileComponents();

		fixture = TestBed.createComponent(TablePartitionsComponent);
	});

	function rendered(): string {
		return fixture.nativeElement.textContent as string;
	}

	// Drains the microtask queue so the awaits inside ngOnInit run to
	// completion, without a timer or a second fetch getting a turn.
	async function settle() {
		for (let i = 0; i < 5; i++) await Promise.resolve();
	}

	it('does not claim the instance is unlicensed while the license read is in flight', () => {
		fixture.detectChanges();

		expect(rendered()).not.toContain(UNLICENSED_COPY);
	});

	it('claims the instance is unlicensed once the license read answers no', async () => {
		licensed = false;
		fixture.detectChanges();

		answerLicense();
		await settle();
		fixture.changeDetectorRef.detectChanges();

		expect(rendered()).toContain(UNLICENSED_COPY);
	});

	it('never claims the instance is unlicensed when the license read answers yes', async () => {
		fixture.detectChanges();

		answerLicense();
		await settle();
		fixture.changeDetectorRef.detectChanges();

		expect(rendered()).not.toContain(UNLICENSED_COPY);
	});
});
