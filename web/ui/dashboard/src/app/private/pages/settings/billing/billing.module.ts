import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { BillingPageComponent } from './billing-page.component';
import { BillingOverviewComponent } from './billing-overview.component';
import { BillingUsageComponent } from './billing-usage.component';
import { BillingInvoicesComponent } from './billing-invoices.component';
import {CardComponent} from "../../../../components/card/card.component";
import {
    TableCellComponent, TableComponent,
    TableHeadCellComponent, TableHeadComponent,
    TableRowComponent
} from "../../../../components/table/table.component";
import {TableLoaderModule} from "../../../components/table-loader/table-loader.module";
import {BadgeComponent} from "../../../../components/badge/badge.component";
import {ButtonComponent} from "../../../../components/button/button.component";
import {DropdownComponent, DropdownOptionDirective} from "../../../../components/dropdown/dropdown.component";
import {PermissionDirective} from "../../../components/permission/permission.directive";
import {RolePipe} from "../../../../pipes/role/role.pipe";

@NgModule({
  declarations: [
    BillingPageComponent,
    BillingOverviewComponent,
    BillingUsageComponent,
    BillingInvoicesComponent
  ],
    imports: [CommonModule, CardComponent, TableRowComponent, TableCellComponent, TableHeadCellComponent, TableComponent, TableHeadComponent, TableLoaderModule, BadgeComponent, ButtonComponent, DropdownComponent, DropdownOptionDirective, PermissionDirective, RolePipe],
  exports: [
    BillingPageComponent,
    BillingOverviewComponent,
    BillingUsageComponent,
    BillingInvoicesComponent
  ]
})
export class BillingModule {}
