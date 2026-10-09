import {Component, OnInit, ViewChild} from '@angular/core';
import {CommonModule} from '@angular/common';
import {EventDeliveriesModule} from 'src/app/private/pages/project/events/event-deliveries/event-deliveries.module';
import {EventDeliveriesComponent as EventDeliveriesTableComponent} from 'src/app/private/pages/project/events/event-deliveries/event-deliveries.component';
import {DatePickerComponent} from '../../components/date-picker/date-picker.component';
import {DropdownComponent, DropdownOptionDirective} from '../../components/dropdown/dropdown.component';
import {SkeletonLoaderComponent} from '../../components/skeleton-loader/skeleton-loader.component';
import {format} from 'date-fns';
import {CHARTDATA, HTTP_RESPONSE, PAGINATION} from '../../models/global.model';
import {EVENT_DELIVERY} from '../../models/event.model';
import {ENDPOINT} from '../../models/endpoint.model';
import {FormBuilder, FormGroup} from '@angular/forms';
import {EventsService} from '../../private/pages/project/events/events.service';
import {PrivateService} from '../../private/private.service';
import {Router} from '@angular/router';
import {LicensesService} from "../../services/licenses/licenses.service";
import {CountLabelPipe} from 'src/app/pipes/count-label/count-label.pipe';

@Component({
    selector: 'convoy-event-deliveries',
    imports: [CommonModule, EventDeliveriesModule, DatePickerComponent, DropdownComponent, DropdownOptionDirective, SkeletonLoaderComponent, CountLabelPipe],
    templateUrl: './event-deliveries.component.html',
    styleUrls: ['./event-deliveries.component.scss']
})
export class EventDeliveriesComponent implements OnInit {
    isloadingDashboardData: boolean = false;
    dashboardFrequency: 'daily' | 'weekly' | 'monthly' | 'yearly' = 'daily';
    filterOptions: ['daily', 'weekly', 'monthly'] = ['daily', 'weekly', 'monthly'];
    dashboardData = {apps: 0, events_sent: 0};
    eventDeliveries!: { pagination: PAGINATION; content: EVENT_DELIVERY[] };
    statsDateRange: FormGroup = this.formBuilder.group({
        startDate: [{value: new Date(new Date().setDate(new Date().getDate() - 30)), disabled: true}],
        endDate: [{value: new Date(), disabled: true}]
    });
    dateRangeValue?: {
        startDate: string | Date;
        endDate: string | Date;
    };
    chartData: CHARTDATA[] = [];
    isPageLoading = false;

    // Success/failure totals for the metric cards and an endpoint scope for
    // those counts. null means unknown, which the cards render as a dash;
    // 0 means the window really held none.
    deliveryCounts: { success: number | null; failure: number | null } = {success: null, failure: null};
    isLoadingDeliveryCounts = false;
    private deliveryCountsFetchId = 0;
    summaryEndpoints: ENDPOINT[] = [];
    selectedSummaryEndpoint?: ENDPOINT;
    isLoadingSummaryEndpoints = false;

    @ViewChild(EventDeliveriesTableComponent) eventDeliveriesTable?: EventDeliveriesTableComponent;

    constructor(private formBuilder: FormBuilder, private eventsService: EventsService, private privateService: PrivateService, public licenseService: LicensesService, public router: Router) {
    }


    async ngOnInit() {
        this.isloadingDashboardData = true;
        this.isPageLoading = true;
        this.fetchDashboardData();
        this.getEndpointsForSummary();
        this.licenseService.setLicenses()
    }

    setDateForFilter(requestDetails: { startDate: Date; endDate: Date; startTime?: string; endTime?: string }) {
        if (!requestDetails.endDate && !requestDetails.startDate) return {startDate: '', endDate: ''};
        const startDate = requestDetails.startDate ? `${format(requestDetails.startDate, 'yyyy-MM-dd')}${requestDetails?.startTime || 'T00:00:00'}` : '';
        const endDate = requestDetails.endDate ? `${format(requestDetails.endDate, 'yyyy-MM-dd')}${requestDetails?.endTime || 'T23:59:59'}` : '';
        return {startDate, endDate};
    }

    // Formatted range for the date button, e.g. "12/02/2026 - 13/03/2026"
    get dateRangeLabel(): string {
        const {startDate, endDate} = this.statsDateRange.getRawValue();
        if (!startDate || !endDate) return 'Select date range';
        return `${format(new Date(startDate), 'dd/MM/yyyy')} - ${format(new Date(endDate), 'dd/MM/yyyy')}`;
    }

    get statsDateRangeForRequest(): { startDate: string; endDate: string } {
        const rawValue = this.statsDateRange.getRawValue();
        return typeof rawValue.startDate !== 'string' ? this.setDateForFilter(rawValue) : rawValue;
    }

    async fetchDashboardData() {
        const {startDate, endDate} = this.statsDateRangeForRequest;

        this.fetchDeliveryCounts();

        try {
            const dashboardResponse = await this.eventsService.dashboardSummary({
                startDate,
                endDate,
                type: this.dashboardFrequency
            });
            this.dashboardData = dashboardResponse.data;
            this.initConvoyChart(dashboardResponse);

            this.isloadingDashboardData = false;
            return;
        } catch (error: any) {
            this.chartData = [];
            this.isloadingDashboardData = false;
            this.isPageLoading = false;
            return;
        }
    }

    // Accurate "Active endpoints" count from the portal-scoped list (the
    // dashboard summary's apps field is total endpoints, including paused).
    get activeEndpointCount(): number {
        return this.summaryEndpoints.filter(endpoint => endpoint.status === 'active').length;
    }

    // Successful/Failed totals, served from the daily rollup. A failed request
    // yields null, which both clears stale cards after a date or endpoint change
    // and avoids claiming zero for a total we do not have.
    async fetchDeliveryCounts() {
        const {startDate, endDate} = this.statsDateRangeForRequest;
        const endpointId = this.selectedSummaryEndpoint?.uid;
        const fetchId = ++this.deliveryCountsFetchId;

        this.isLoadingDeliveryCounts = true;
        const counts = await this.eventsService.getSummaryDeliveryCounts({startDate, endDate, endpointId});

        // Only the latest request may write the cards or clear the flag, or an
        // earlier reply overwrites newer totals and hides the running spinner.
        if (fetchId !== this.deliveryCountsFetchId) return;
        this.deliveryCounts = counts;
        this.isLoadingDeliveryCounts = false;
    }

    async getEndpointsForSummary() {
        this.isLoadingSummaryEndpoints = true;
        try {
            // Walk all pages so the scope dropdown includes every endpoint the
            // portal link can see, not just the first page.
            this.summaryEndpoints = await this.privateService.getAllEndpoints({ perPage: 100 });
        } catch (error) {
            this.summaryEndpoints = [];
        } finally {
            this.isLoadingSummaryEndpoints = false;
        }
    }

    selectSummaryEndpoint(endpoint?: ENDPOINT) {
        this.selectedSummaryEndpoint = endpoint;
        this.fetchDeliveryCounts();
    }

    getSelectedDateRange(dateRange?: { startDate: Date; endDate: Date }) {
        this.dateRangeValue = dateRange;
        this.statsDateRange.patchValue({
            startDate: dateRange?.startDate || new Date(new Date().setDate(new Date().getDate() - 30)),
            endDate: dateRange?.endDate || new Date()
        });
        this.fetchDashboardData();

        // The page-level date range also scopes the delivery table below.
        // Always pass explicit dates: clearing resets to the default 30-day
        // window, and without dates the list API silently narrows to 7 days.
        const {startDate, endDate} = this.statsDateRangeForRequest;
        this.eventDeliveriesTable?.applyDateFilter({startDate, endDate});
    }

    initConvoyChart(dashboardResponse: HTTP_RESPONSE) {
        // Guard missing/failed payloads so the template's @for never iterates undefined.
        const rawBuckets = dashboardResponse?.data?.event_data;
        if (!Array.isArray(rawBuckets)) {
            this.chartData = [];
            return;
        }

        // Sort ascending so the oldest bucket sits on the left and today on the far right.
        const eventData = [...rawBuckets].sort((a: any, b: any) => new Date(a.data.date).getTime() - new Date(b.data.date).getTime());
        const labelFormat = this.getDateLabelFormat();
        this.chartData = eventData.map((data: any) => ({
            label: format(new Date(data.data.date), labelFormat),
            data: data.count || 0
        }));
    }

    get maxChartValue(): number {
        return Math.max(...(this.chartData || []).map(bucket => bucket.data), 1);
    }

    barHeight(count: number): number {
        // Keep a sliver visible for zero-count buckets, like the design.
        return Math.max((count / this.maxChartValue) * 100, 1.5);
    }

    getDateLabelFormat() {
        let labelsDateFormat = '';
        switch (this.dashboardFrequency) {
            case 'daily':
            case 'weekly':
                labelsDateFormat = 'dd-MM';
                break;
            case 'monthly':
                labelsDateFormat = 'MMM yy';
                break;
            case 'yearly':
                labelsDateFormat = 'yyyy';
                break;
            default:
                break;
        }

        return labelsDateFormat;
    }

}
